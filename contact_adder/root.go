package contact_adder

import (
	"MailContactUtilty/helper"

	"golang.org/x/net/context"
	"google.golang.org/api/option"
	"google.golang.org/api/people/v1"
)

type ContactAdder struct {
	*people.Service
}

func NewContactAdder(clientOption option.ClientOption) (*ContactAdder, error) {
	srv, err := people.NewService(context.Background(), clientOption)
	if err != nil {
		return nil, err

	}
	return &ContactAdder{
		Service: srv,
	}, nil
}

func (ca *ContactAdder) AddContact(contact *helper.Contact) (*helper.Contact, error) {
	_, err := ca.People.CreateContact(&people.Person{
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
		return nil, err
	}
	return contact, nil
}
