package mail_reciever

import (
	"MailContactUtilty/google_auth"
	"MailContactUtilty/helper"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"cloud.google.com/go/pubsub"
	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/option"
)

type MailReciever struct {
	*gmail.Service
	Email        string
	PubSubClient *pubsub.Client
	projectId    string
}

type PubSubMessage struct {
	Email     string `json:"emailAddress"`
	HistoryId uint64 `json:"historyId"`
}

func (mr *MailReciever) Reply(ctx context.Context, id string, contact *helper.Contact, originalMsg *gmail.Message, sender string) error {
	var subject string
	for _, header := range originalMsg.Payload.Headers {
		if header.Name == "Subject" {
			subject = header.Value
		}
	}
	rawMessage := "From: " + mr.Email + "\r\n" +
		"To: " + sender + "\r\n" +
		"Subject: Re: " + subject + "\r\n" +
		"References: " + originalMsg.Id + "\r\n" +
		"In-Reply-To: " + originalMsg.Id + "\r\n" +
		"Content-Type: text/plain; charset=UTF-8\r\n\r\n" +
		"Thank you for your email. I've added the following contact information:\n" +
		"Name: " + contact.Name + "\n" +
		"Surname: " + contact.Surname + "\n" +
		"Email: " + contact.Email + "\n" +
		"Phone: " + contact.Phone + "\n" +
		"Organization: " + contact.Organization + "\n"

	message := &gmail.Message{
		Raw:      base64.URLEncoding.EncodeToString([]byte(rawMessage)),
		ThreadId: originalMsg.ThreadId,
	}

	_, err := mr.Service.Users.Messages.Send("me", message).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("unable to send reply: %v", err)
	}

	return nil
}

func NewMailReciever(ctx context.Context, httpOption option.ClientOption, authConfig google_auth.AuthConfig, projectId string) (*MailReciever, error) {
	srv, err := gmail.NewService(ctx, httpOption)
	if err != nil {
		log.Printf("Unable to create people Client %v", err)
		return nil, err
	}
	pubSubClient, err := pubsub.NewClient(ctx, projectId)
	if err != nil {
		log.Printf("Unable to create pubsub client %v", err)
		return nil, err
	}
	return &MailReciever{Service: srv, Email: authConfig.Email, PubSubClient: pubSubClient, projectId: projectId}, nil
}

func (mr *MailReciever) GetMessages(ctx context.Context) ([]*gmail.Message, error) {
	messages, err := mr.Service.Users.Messages.List("me").Context(ctx).Do()
	if err != nil {
		log.Printf("Unable to retrieve messages: %v, email: %s", err, mr.Email)
		return nil, err
	}
	return messages.Messages, nil
}

func (mr *MailReciever) GetUnreadMessages(ctx context.Context) ([]*gmail.Message, error) {
	messages, err := mr.Service.Users.Messages.List("me").Q("is:unread").Context(ctx).Do()
	if err != nil {
		log.Printf("Unable to retrieve messages: %v, email: %s", err, mr.Email)
		return nil, err
	}
	return messages.Messages, nil
}

const (
	maxRetries = 3
	retryDelay = 5 * time.Second
)

func (mr *MailReciever) ListenForEmails(ctx context.Context, target chan<- *gmail.Message) error {
	messageAlert := make(chan PubSubMessage)

	_, err := mr.ensureTopic(ctx)
	if err != nil {
		return fmt.Errorf("topic setup failed: %w", err)
	}

	if err := mr.setupWatch(ctx); err != nil {
		return fmt.Errorf("watch setup failed: %w", err)
	}

	sub, err := mr.ensureSubscription(ctx)
	if err != nil {
		return fmt.Errorf("subscription setup failed: %w", err)
	}

	log.Printf("Starting to receive messages...")
	go func() {
		for {
			err := sub.Receive(ctx, func(msgCtx context.Context, m *pubsub.Message) {
				var pubSubMessage PubSubMessage
				if err := json.Unmarshal(m.Data, &pubSubMessage); err != nil {
					log.Printf("Unable to unmarshal message: %v", err)
					m.Ack()
					return
				}
				select {
				case messageAlert <- pubSubMessage:
				case <-ctx.Done():
					return
				}
				m.Ack()
			})
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				log.Printf("Receive error: %v, retrying in %v", err, retryDelay)
				time.Sleep(retryDelay)
				continue
			}
		}
	}()

	for {
		log.Println("Checking for new emails...")
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-messageAlert:
			if err := mr.handleNewMessages(ctx, target); err != nil {
				log.Printf("Error handling messages: %v", err)
			}
		}
	}
}

func (mr *MailReciever) ensureTopic(ctx context.Context) (bool, error) {
	exists, err := mr.PubSubClient.Topic("gmail-watcher").Exists(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to check topic existence: %w", err)
	}

	if !exists {
		_, err = mr.PubSubClient.CreateTopic(ctx, "gmail-watcher")
		if err != nil {
			return false, fmt.Errorf("failed to create topic: %w", err)
		}
		log.Printf("Created new topic: gmail-watcher")
	}
	return exists, nil
}

func (mr *MailReciever) setupWatch(ctx context.Context) error {
	for i := 0; i < maxRetries; i++ {
		_, err := mr.Service.Users.Watch("me", &gmail.WatchRequest{
			LabelIds:  []string{"INBOX"},
			TopicName: fmt.Sprintf("projects/%s/topics/gmail-watcher", mr.projectId),
		}).Context(ctx).Do()
		if err == nil {
			return nil
		}
		if i < maxRetries-1 {
			log.Printf("Watch setup failed (attempt %d/%d): %v", i+1, maxRetries, err)
			time.Sleep(retryDelay)
			continue
		}
		return fmt.Errorf("failed to set up watch after %d attempts: %w", maxRetries, err)
	}
	return nil
}

func (mr *MailReciever) ensureSubscription(ctx context.Context) (*pubsub.Subscription, error) {
	sub := mr.PubSubClient.Subscription("gmail-watcher-sub")
	exists, err := sub.Exists(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to check subscription existence: %w", err)
	}

	if !exists {
		log.Printf("Creating new subscription: %s", "gmail-watcher-sub")
		sub, err = mr.PubSubClient.CreateSubscription(ctx, "gmail-watcher-sub", pubsub.SubscriptionConfig{
			Topic:            mr.PubSubClient.Topic("gmail-watcher"),
			AckDeadline:      10 * time.Second,
			ExpirationPolicy: 24 * time.Hour,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create subscription: %w", err)
		}
		log.Printf("Successfully created subscription")
	} else {
		log.Printf("Using existing subscription: %s", "gmail-watcher-sub")
	}
	return sub, nil
}

func (mr *MailReciever) handleNewMessages(ctx context.Context, target chan<- *gmail.Message) error {
	newMessages, err := mr.GetUnreadMessages(ctx)
	if err != nil {
		return fmt.Errorf("error fetching messages: %w", err)
	}

	for _, msg := range newMessages {
		fullMsg, err := mr.GetMessage(ctx, msg.Id)
		if err != nil {
			log.Printf("Error fetching message details: %v", err)
			continue
		}

		log.Printf("New email received - Subject: %s, From: %s",
			getHeader(fullMsg.Payload.Headers, "Subject"),
			getHeader(fullMsg.Payload.Headers, "From"))

		select {
		case target <- fullMsg:
		case <-ctx.Done():
			return ctx.Err()
		}

		if err := mr.MarkAsRead(ctx, msg.Id); err != nil {
			log.Printf("Failed to mark message %s as read: %v", msg.Id, err)
		}
	}
	return nil
}

func (mr *MailReciever) MarkAsRead(ctx context.Context, id string) error {
	_, err := mr.Service.Users.Messages.Modify("me", id, &gmail.ModifyMessageRequest{
		RemoveLabelIds: []string{"UNREAD"},
	}).Context(ctx).Do()
	if err != nil {
		log.Printf("Unable to mark message as read: %v", err)
		return err
	}
	return nil
}

func getHeader(headers []*gmail.MessagePartHeader, name string) string {
	for _, header := range headers {
		if header.Name == name {
			return header.Value
		}
	}
	return ""
}

func (mr *MailReciever) GetMessage(ctx context.Context, id string) (*gmail.Message, error) {
	msg, err := mr.Service.Users.Messages.Get("me", id).Context(ctx).Do()
	if err != nil {
		log.Printf("Unable to retrieve message: %v", err)
		return nil, err
	}
	return msg, nil
}

func (mr *MailReciever) GetAttachment(ctx context.Context, messageId, attachmentId string) (*gmail.MessagePartBody, error) {
	msg, err := mr.Service.Users.Messages.Attachments.Get("me", messageId, attachmentId).Context(ctx).Do()
	if err != nil {
		log.Printf("Unable to retrieve attachment: %v", err)
		return nil, err
	}
	return msg, nil
}

func (mr *MailReciever) GetEmail(ctx context.Context) (string, error) {
	profile, err := mr.Service.Users.GetProfile("me").Context(ctx).Do()
	if err != nil {
		log.Printf("Unable to retrieve user profile: %v", err)
		return "", err
	}
	return profile.EmailAddress, nil
}
