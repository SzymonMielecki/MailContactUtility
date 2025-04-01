package server

import (
	"MailContactUtilty/contact_adder"
	"MailContactUtilty/contact_generator"
	"MailContactUtilty/database"
	"MailContactUtilty/google_auth"
	"MailContactUtilty/mail_reciever"
	"MailContactUtilty/web_handler"
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"net"
	"net/http"
	"regexp"
	"slices"
	"time"

	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/option"
	"google.golang.org/api/people/v1"
)

type Server struct {
	AuthClient    *google_auth.Auth
	MailClient    *mail_reciever.MailReciever
	ContactClient *contact_generator.ContactGenerator
	WebServer     *http.Server
	ctx           context.Context
	cancel        context.CancelFunc
	errChan       chan error
	MailList      chan *gmail.Message
	projectID     string
}

func NewServer(dbConfig database.DatabaseConfig, projectID string) (*Server, error) {
	ctx, cancel := context.WithCancel(context.Background())
	auth, err := google_auth.NewAuth(ctx, dbConfig)
	if err != nil {
		cancel()
		return nil, err
	}
	return &Server{
		projectID:     projectID,
		AuthClient:    auth,
		errChan:       make(chan error, 1),
		ctx:           ctx,
		cancel:        cancel,
		MailList:      make(chan *gmail.Message),
		ContactClient: contact_generator.NewContactGenerator(),
	}, nil
}
func (s *Server) Start(authConfig *google_auth.AuthConfig) {
	sm := http.NewServeMux()
	sm.Handle("/register", web_handler.Register(s.AuthClient))
	sm.Handle("/auth", web_handler.Auth(s.AuthClient))
	s.AuthClient.StartAuth(authConfig)
	s.MailClient = mail_reciever.NewMailReciever(option.WithHTTPClient(s.AuthClient.GetClient(authConfig)), *authConfig, s.projectID, s.ctx)
	sm.Handle("/email", web_handler.HandleEmail(s.MailClient, s.MailList))
	s.WebServer = &http.Server{
		Addr:        ":8080",
		Handler:     sm,
		BaseContext: func(_ net.Listener) context.Context { return s.ctx },
	}
	go s.ServeWeb()
	go s.MailClient.ListenForEmails(10*time.Second, s.MailList, s.projectID)
	s.Run()
}
func (s *Server) Close() {
	s.ContactClient.Close()
	s.cancel()
}
func (s *Server) ServeWeb() {
	fmt.Println("Server started on :8080")
	if err := s.WebServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		s.errChan <- err
	}
	close(s.errChan)
}
func (s *Server) HandleEmail(mail *gmail.Message) error {
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
	log.Println("Processing email from: ", sender)
	emails, err := s.AuthClient.GetEmails()
	if err != nil {
		return err
	}
	if !slices.Contains(emails, sender) {
		return fmt.Errorf("email not from sender: %s, from: %s", emails, sender)
	}
	authConfig := google_auth.AuthConfig{Email: sender, Scopes: []string{people.ContactsScope}}
	user_auth := option.WithHTTPClient(s.AuthClient.GetClient(&authConfig))
	client_ca := contact_adder.NewContactAdder(user_auth)

	mailContent, err := s.MailClient.GetMessage(mail.Id)
	if err != nil {
		return err
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
	contact := s.ContactClient.Generate(fullMailText)
	log.Println(contact)
	client_ca.AddContact(contact)
	s.MailClient.Reply(mailContent.Id, contact)
	return nil
}

func (s *Server) Run() {
	for {
		select {
		case mail := <-s.MailList:
			if err := s.HandleEmail(mail); err != nil {
				s.errChan <- err
			}
		case err := <-s.errChan:
			log.Printf("Server error: %v\n", err)
			s.cancel()
			return
		case <-s.ctx.Done():
			log.Println("Shutting down gracefully...")
			shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer shutdownCancel()

			s.cancel()
			if err := s.WebServer.Shutdown(shutdownCtx); err != nil {
				log.Printf("Server shutdown error: %v\n", err)
			}
			return
		}

	}
}
