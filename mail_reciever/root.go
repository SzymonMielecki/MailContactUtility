package mail_reciever

import (
	"context"
	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/option"
	"log"
)

type MailReciever struct {
	service *gmail.Service
}

func NewMailReciever(clientOption option.ClientOption) *MailReciever {
	srv, err := gmail.NewService(context.TODO(), clientOption)
	if err != nil {
		log.Fatalf("Unable to create people Client %v", err)
	}
	return &MailReciever{service: srv}
}

func (mr *MailReciever) GetMessages() ([]*gmail.Message, error) {
	userId := "me"
	messages, err := mr.service.Users.Messages.List(userId).Do()
	if err != nil {
		log.Fatalf("Unable to retrieve messages: %v", err)
		return nil, err
	}
	return messages.Messages, nil
}

func (mr *MailReciever) GetMessage(id string) (*gmail.Message, error) {
	userId := "me"
	msg, err := mr.service.Users.Messages.Get(userId, id).Do()
	if err != nil {
		log.Fatalf("Unable to retrieve message: %v", err)
		return nil, err
	}
	return msg, nil

}
