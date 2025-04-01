package mail_reciever

import (
	"MailContactUtilty/google_auth"
	"MailContactUtilty/helper"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"cloud.google.com/go/pubsub"
	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/option"
)

type MailReciever struct {
	PubSubClient  *pubsub.Client
	Topic         *pubsub.Topic
	Service       *gmail.Service
	Email         string
	ctx           context.Context
	lastHistoryId uint64
	history       []*gmail.History
}

type PubSubMessage struct {
	Email     string `json:"emailAddress"`
	HistoryId uint64 `json:"historyId"`
}

func NewMailReciever(clientOption option.ClientOption, authConfig google_auth.AuthConfig, projectID string, ctx context.Context) *MailReciever {
	pubSubClient, err := pubsub.NewClient(ctx, projectID)
	if err != nil {
		log.Fatalf("Unable to create pubsub client: %v", err)
	}
	srv, err := gmail.NewService(ctx, clientOption)
	if err != nil {
		log.Printf("Unable to create people Client %v", err)
	}

	topicName := "mail_reciever-" + authConfig.Email[:strings.Index(authConfig.Email, "@")]
	topic := pubSubClient.Topic(topicName)
	exists, err := topic.Exists(context.Background())
	if err != nil {
		log.Fatalf("Unable to check if topic exists: %v", err)
	}

	if !exists {
		topic, err = pubSubClient.CreateTopic(context.Background(), topicName)
		if err != nil {
			log.Fatalf("Unable to create topic: %v", err)
		}
	}
	mr := &MailReciever{PubSubClient: pubSubClient, Topic: topic, Service: srv, Email: authConfig.Email, ctx: ctx}
	_, err = mr.GetMessages()
	if err != nil {
		log.Fatalf("Unable to get messages: %v", err)
	}
	return mr
}

func (mr *MailReciever) Reply(id string, contact helper.Contact) error {
	originalMsg, err := mr.GetMessage(id)
	if err != nil {
		return fmt.Errorf("unable to get original message: %v", err)
	}

	emailContent := fmt.Sprintf("Subject: Re: Contact Added\r\n"+
		"References: %s\r\n"+
		"In-Reply-To: %s\r\n"+
		"Content-Type: text/plain; charset=UTF-8\r\n\r\n"+
		"Thank you for your email. I've added the following contact information:\n"+
		"Name: %s\n"+
		"Surname: %s\n"+
		"Email: %s\n"+
		"Phone: %s",
		originalMsg.Id,
		originalMsg.Id,
		contact.Name,
		contact.Surname,
		contact.Email,
		contact.Phone)

	message := &gmail.Message{
		Raw:      base64.URLEncoding.EncodeToString([]byte(emailContent)),
		ThreadId: originalMsg.ThreadId,
	}

	_, err = mr.Service.Users.Messages.Send("me", message).Do()
	if err != nil {
		return fmt.Errorf("unable to send reply: %v", err)
	}

	return nil
}

func (mr *MailReciever) GetMessages() ([]*gmail.Message, error) {
	history, err := mr.Service.Users.History.List("me").StartHistoryId(mr.lastHistoryId).Do()
	if err != nil {
		log.Printf("Unable to retrieve messages: %v, email: %s", err, mr.Email)
		return nil, err
	}
	mr.lastHistoryId = history.HistoryId
	mr.history = history.History

	return messages, nil
}

func (mr *MailReciever) ListenForEmails(pollInterval time.Duration, messageChan chan<- *gmail.Message, projectID string) error {
	log.Printf("Setting up Gmail watch for inbox: %s", mr.Email)
	_, err := mr.Service.Users.Watch("me", &gmail.WatchRequest{
		LabelIds:  []string{"INBOX"},
		TopicName: "projects/" + projectID + "/topics/mail_reciever-" + mr.Email[:strings.Index(mr.Email, "@")],
	}).Context(mr.ctx).Do()
	if err != nil {
		return fmt.Errorf("failed to set up Gmail watch: %v", err)
	}
	log.Printf("Successfully set up Gmail watch for inbox")

	subscriptionName := "mail_reciever-" + mr.Email[:strings.Index(mr.Email, "@")] + "-sub"
	sub := mr.PubSubClient.Subscription(subscriptionName)
	exists, err := sub.Exists(context.Background())
	if err != nil {
		return fmt.Errorf("failed to check subscription existence: %v", err)
	}

	if !exists {
		log.Printf("Creating new subscription: %s", subscriptionName)
		sub, err = mr.PubSubClient.CreateSubscription(mr.ctx, subscriptionName, pubsub.SubscriptionConfig{
			Topic:            mr.Topic,
			AckDeadline:      10 * time.Second,
			ExpirationPolicy: 24 * time.Hour,
		})
		if err != nil {
			return fmt.Errorf("failed to create subscription: %v", err)
		}
		log.Printf("Successfully created subscription")
	} else {
		log.Printf("Using existing subscription: %s", subscriptionName)
	}

	log.Printf("Starting to receive messages...")
	err = sub.Receive(mr.ctx, func(ctx context.Context, m *pubsub.Message) {
		var pubSubMessage PubSubMessage
		fmt.Println("Received message:", string(m.Data))
		if err := json.Unmarshal(m.Data, &pubSubMessage); err != nil {
			log.Printf("Unable to unmarshal message: %v", err)
			m.Ack()
			return
		}
		fmt.Println("Received message:", pubSubMessage)
		message, err := mr.GetMessageByHistory(pubSubMessage.HistoryId)
		if err != nil {
			log.Printf("Unable to get message: %v", err)
			m.Ack()
			return
		}
		messageChan <- message
		m.Ack()
	})
	if err != nil {
		return err
	}
	return nil
}

func (mr *MailReciever) GetMessageByHistory(historyId uint64) (*gmail.Message, error) {
	for _, history := range mr.history {
		if history.Id == historyId {
			return mr.GetMessage(history.Messages[len(history.Messages)-1].Id)
		}
	}
	return nil, fmt.Errorf("message not found for history ID: %d", historyId)
}

func (mr *MailReciever) GetMessage(id string) (*gmail.Message, error) {
	msg, err := mr.Service.Users.Messages.Get("me", id).Do()
	if err != nil {
		log.Printf("Unable to retrieve message: %v", err)
		return nil, err
	}
	return msg, nil

}
