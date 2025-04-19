package contact_adder

import (
	"MailContactUtilty/helper"
	"context"

	"google.golang.org/api/option"
	"google.golang.org/api/people/v1"
)

type ContactAdder struct {
	*people.Service
}

func NewContactAdder(ctx context.Context, clientOption option.ClientOption) (*ContactAdder, error) {
	srv, err := people.NewService(ctx, clientOption)
	if err != nil {
		return nil, err
	}
	return &ContactAdder{
		Service: srv,
	}, nil
}

func (ca *ContactAdder) AddContact(ctx context.Context, contact *helper.Contact) (*helper.Contact, error) {
	surname := contact.Surname
	exists, err := ca.CheckExists(ctx, contact)
	if err != nil {
		return nil, err
	}
	if exists {
		surname = surname + " DUPLICATE"
	}

	_, err = ca.People.CreateContact(&people.Person{
		Names: []*people.Name{
			{
				GivenName:  contact.Name,
				FamilyName: surname,
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
	}).Context(ctx).Do()
	if err != nil {
		return nil, err
	}
	return contact, nil
}

func (ca *ContactAdder) CheckExists(ctx context.Context, contact *helper.Contact) (bool, error) {
	resp, err := ca.People.SearchContacts().Query(contact.Name + " " + contact.Surname).ReadMask("names").Context(ctx).Do()
	if err != nil {
		return false, err
	}
	return len(resp.Results) > 0, nil
}
