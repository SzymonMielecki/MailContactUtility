package main

import (
	"MailContactUtilty/contact_adder"
	"MailContactUtilty/contact_generator"
	"MailContactUtilty/google_auth"
	"MailContactUtilty/mail_reciever"
	"fmt"
	"github.com/joho/godotenv"
	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/option"
	"google.golang.org/api/people/v1"
	"io"
	"log"
	"os"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Fatal(err)
	}
	auth := option.WithHTTPClient(google_auth.GetClient("credentials.json", []string{people.ContactsScope, gmail.GmailReadonlyScope}))
	client_cg := contact_generator.NewClient()
	defer client_cg.Close()
	file, err := os.Open("test_mail.txt")
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()
	b, err := io.ReadAll(file)
	if err != nil {
		log.Fatal(err)
	}
	contact := client_cg.Generate(string(b[:]))
	client_ca := contact_adder.NewContactAdder(auth)
	client_mr := mail_reciever.NewMailReciever(auth)
	messages, err := client_mr.GetMessages()
	if err != nil {
		log.Fatal(err)
	}
	for _, message := range messages {
		msg, err := client_mr.GetMessage(message.Id)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println(msg)
	}
	contact, err = client_ca.AddContact(contact)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(contact)
}
