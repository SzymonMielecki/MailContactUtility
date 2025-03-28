package mail_reciever

import (
	"MailContactUtilty/google_auth"
	"MailContactUtilty/helper"
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"time"

	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/option"
)

type MailReciever struct {
	*gmail.Service
	Email string
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

func NewMailReciever(clientOption option.ClientOption, authConfig google_auth.AuthConfig) *MailReciever {
	srv, err := gmail.NewService(context.TODO(), clientOption)
	if err != nil {
		log.Printf("Unable to create people Client %v", err)
	}
	return &MailReciever{Service: srv, Email: authConfig.Email}
}

func (mr *MailReciever) GetMessages() ([]*gmail.Message, error) {
	userId := "me"
	messages, err := mr.Service.Users.Messages.List(userId).Do()
	if err != nil {
		log.Printf("Unable to retrieve messages: %v, email: %s", err, mr.Email)
		return nil, err
	}
	return messages.Messages, nil
}

func (mr *MailReciever) ListenForEmails(ctx context.Context, pollInterval time.Duration, channel chan<- *gmail.Message) error {
	knownMessageIds := make(map[string]bool)

	messages, err := mr.GetMessages()
	if err != nil {
		return err
	}
	for _, msg := range messages {
		knownMessageIds[msg.Id] = true
	}

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			log.Println("Checking for new emails...")
			newMessages, err := mr.GetMessages()
			if err != nil {
				log.Printf("Error fetching messages: %v", err)
				continue
			}

			for _, msg := range newMessages {
				if !knownMessageIds[msg.Id] {
					fullMsg, err := mr.GetMessage(msg.Id)
					if err != nil {
						log.Printf("Error fetching message details: %v", err)
						continue
					}

					to := getHeader(fullMsg.Payload.Headers, "To")
					if to != "contacterutil@gmail.com" {
						log.Printf("Skipping email not sent to contacterutil@gmail.com: %s", to)
						continue
					}

					log.Printf("New email received - Subject: %s, From: %s", getHeader(fullMsg.Payload.Headers, "Subject"), getHeader(fullMsg.Payload.Headers, "From"))
					channel <- fullMsg
					knownMessageIds[msg.Id] = true
				}
			}
		}
	}
}

func getHeader(headers []*gmail.MessagePartHeader, name string) string {
	for _, header := range headers {
		if header.Name == name {
			return header.Value
		}
	}
	return ""
}

func (mr *MailReciever) GetMessage(id string) (*gmail.Message, error) {
	userId := "me"
	msg, err := mr.Service.Users.Messages.Get(userId, id).Do()
	if err != nil {
		log.Printf("Unable to retrieve message: %v", err)
		return nil, err
	}
	return msg, nil

}

func (mr *MailReciever) GetEmail() (string, error) {
	profile, err := mr.Service.Users.GetProfile("me").Do()
	if err != nil {
		log.Fatalf("Unable to retrieve user profile: %v", err)
		return "", err
	}
	return profile.EmailAddress, nil
}
