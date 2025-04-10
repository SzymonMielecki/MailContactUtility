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
		"Phone:" + contact.Phone + "\n" +
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

func (mr *MailReciever) ListenForEmails(ctx context.Context, target chan<- *gmail.Message) error {
	messageAlert := make(chan PubSubMessage)

	exists, err := mr.PubSubClient.Topic("gmail-watcher").Exists(ctx)
	if err != nil {
		return fmt.Errorf("unable to check if topic exists: %v", err)
	}

	if !exists {
		_, err = mr.PubSubClient.CreateTopic(ctx, "gmail-watcher")
		if err != nil {
			return fmt.Errorf("unable to create topic: %v", err)
		}
	}

	_, err = mr.Service.Users.Watch("me", &gmail.WatchRequest{
		LabelIds:  []string{"INBOX"},
		TopicName: fmt.Sprintf("projects/%s/topics/gmail-watcher", mr.projectId),
	}).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("unable to watch for emails: %v", err)
	}

	sub := mr.PubSubClient.Subscription("gmail-watcher-sub")
	exists, err = sub.Exists(ctx)
	if err != nil {
		return fmt.Errorf("failed to check subscription existence: %v", err)
	}

	if !exists {
		log.Printf("Creating new subscription: %s", "gmail-watcher-sub")
		sub, err = mr.PubSubClient.CreateSubscription(ctx, "gmail-watcher-sub", pubsub.SubscriptionConfig{
			Topic:            mr.PubSubClient.Topic("gmail-watcher"),
			AckDeadline:      10 * time.Second,
			ExpirationPolicy: 24 * time.Hour,
		})
		if err != nil {
			return fmt.Errorf("failed to create subscription: %v", err)
		}
		log.Printf("Successfully created subscription")
	} else {
		log.Printf("Using existing subscription: %s", "gmail-watcher-sub")
	}

	log.Printf("Starting to receive messages...")
	go func() {
		err = sub.Receive(ctx, func(msgCtx context.Context, m *pubsub.Message) {
			var pubSubMessage PubSubMessage
			if err := json.Unmarshal(m.Data, &pubSubMessage); err != nil {
				log.Printf("Unable to unmarshal message: %v", err)
				m.Ack()
				return
			}
			messageAlert <- pubSubMessage
			m.Ack()
		})
	}()

	for {
		log.Println("Checking for new emails...")
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-messageAlert:
			log.Println("Checking for new emails...")
			newMessages, err := mr.GetUnreadMessages(ctx)
			if err != nil {
				log.Printf("Error fetching messages: %v", err)
				continue
			}

			for _, msg := range newMessages {
				fullMsg, err := mr.GetMessage(ctx, msg.Id)
				if err != nil {
					log.Printf("Error fetching message details: %v", err)
					continue
				}

				log.Printf("New email received - Subject: %s, From: %s", getHeader(fullMsg.Payload.Headers, "Subject"), getHeader(fullMsg.Payload.Headers, "From"))
				target <- fullMsg
				mr.MarkAsRead(ctx, msg.Id)
			}
		}
	}
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
