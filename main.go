package main

import (
	"MailContactUtilty/contact_adder"
	"MailContactUtilty/contact_generator"
	"MailContactUtilty/google_auth"
	"MailContactUtilty/mail_reciever"
	"MailContactUtilty/web_handler"
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"regexp"
	"slices"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/option"
	"google.golang.org/api/people/v1"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Fatal(err)
	}

	client_cg := contact_generator.NewClient()
	defer client_cg.Close()

	ctx, cancel := context.WithCancel(context.Background())
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	mailList := make(chan *gmail.Message)

	// Create server with base context
	s := http.Server{
		Addr:        ":8080",
		BaseContext: func(_ net.Listener) context.Context { return ctx },
	}

	http.HandleFunc("/register", web_handler.Register)
	http.HandleFunc("/auth", web_handler.Auth)

	// Start server in goroutine
	serverErr := make(chan error, 1)
	go func() {
		fmt.Println("Server started on :8080")
		if err := s.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErr <- err
		}
		close(serverErr)
	}()

	google_auth.StartAuth("contacterutil@gmail.com", []string{gmail.GmailReadonlyScope, gmail.GmailModifyScope, people.ContactsScope})

	fmt.Println("Authorization completed")
	server_auth := option.WithHTTPClient(google_auth.GetClient("contacterutil@gmail.com", []string{gmail.GmailReadonlyScope, gmail.GmailModifyScope, people.ContactsScope}))
	client_mr := mail_reciever.NewMailReciever(server_auth, "contacterutil@gmail.com")
	go func() {
		pollInterval := 10 * time.Second
		if err := client_mr.ListenForEmails(ctx, pollInterval, mailList); err != nil {
			log.Printf("Email listening service stopped: %v", err)
		}
	}()
	for {
		select {
		case mail := <-mailList:
			var sender string
			for _, header := range mail.Payload.Headers {
				if header.Name == "From" {
					emailRegexp := regexp.MustCompile(`<([^>]+)>`)
					matches := emailRegexp.FindStringSubmatch(header.Value)
					if len(matches) > 1 {
						sender = matches[1]
					}
					break
				}
			}
			emails, err := google_auth.GetEmails()
			if err != nil {
				log.Printf("Error getting emails: %v", err)
				continue
			}
			if !slices.Contains(emails, sender) {
				log.Printf("Email not from sender: %s, from: %s", emails, sender)
				continue
			}
			user_auth := option.WithHTTPClient(google_auth.GetClient(sender, []string{gmail.GmailReadonlyScope}))
			client_ca := contact_adder.NewContactAdder(user_auth)

			log.Println("Processing email...")
			mailContent, err := client_mr.GetMessage(mail.Id)
			if err != nil {
				log.Printf("Error getting message: %v", err)
				continue
			}
			fullMailText := ""
			for _, part := range mailContent.Payload.Parts {
				if part.MimeType == "text/plain" {
					mailString, err := base64.URLEncoding.DecodeString(part.Body.Data)
					if err != nil {
						log.Printf("Error decoding message: %v", err)
						continue
					}
					fullMailText += string(mailString)
				}
			}
			fmt.Println(fullMailText)
			contact := client_cg.Generate(fullMailText)
			fmt.Println(contact)
			client_ca.AddContact(contact)
			client_mr.Reply(mailContent.Id, contact)
		case err := <-serverErr:
			log.Printf("Server error: %v\n", err)
			cancel()
			return
		case <-sigChan:
			log.Println("Shutting down gracefully...")
			// Create shutdown context with timeout
			shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer shutdownCancel()

			// Cancel main context and shutdown server
			cancel()
			if err := s.Shutdown(shutdownCtx); err != nil {
				log.Printf("Server shutdown error: %v\n", err)
			}

			return
		}

	}

}
