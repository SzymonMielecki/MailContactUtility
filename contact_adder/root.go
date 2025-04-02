package contact_adder

import (
	"MailContactUtilty/helper"
	"log"

	"golang.org/x/net/context"
	"google.golang.org/api/option"
	"google.golang.org/api/people/v1"
)

type ContactAdder struct {
	*people.Service
}

func NewContactAdder(clientOption option.ClientOption) *ContactAdder {
	srv, err := people.NewService(context.Background(), clientOption)
	if err != nil {
		log.Fatalf("Unable to create people Client %v", err)
	}
	return &ContactAdder{
		Service: srv,
	}
}

func (ca *ContactAdder) AddContact(contact helper.Contact) (helper.Contact, error) {
	p, err := ca.People.CreateContact(&people.Person{
		Names: []*people.Name{
			{
				GivenName:  contact.Name,
				FamilyName: contact.Surname,
			},
		},
		EmailAddresses: []*people.EmailAddress{
			{
				Value: contact.Email,
			},
		},
		PhoneNumbers: []*people.PhoneNumber{
			{
				Value: contact.Phone,
			},
		},
		Organizations: []*people.Organization{
			{
				Name: contact.Organization,
			},
		},
	}).Do()
	if err != nil {
		log.Fatalf("Unable to create contact: %v", err)
		return helper.Contact{}, err
	}
	return helper.Contact{
		Name:    p.Names[0].GivenName,
		Surname: p.Names[0].FamilyName,
		Email:   p.EmailAddresses[0].Value,
		Phone:   p.PhoneNumbers[0].Value,
	}, nil
}
