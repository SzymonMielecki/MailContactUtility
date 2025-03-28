package server

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
	"regexp"
	"slices"
	"time"

	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/option"
	"google.golang.org/api/people/v1"
)

type Server struct {
	client_mr *mail_reciever.MailReciever
	client_cg *contact_generator.ContactGenerator
	webServer *http.Server
	ctx       context.Context
	cancel    context.CancelFunc
	errChan   chan error
	mailList  chan *gmail.Message
}

func NewServer() *Server {
	ctx, cancel := context.WithCancel(context.Background())
	return &Server{
		errChan: make(chan error, 1),
		ctx:     ctx,
		cancel:  cancel,
	}
}
func (s *Server) Start(authConfig *google_auth.AuthConfig) {
	sm := http.NewServeMux()
	sm.HandleFunc("/register", web_handler.Register)
	sm.HandleFunc("/auth", web_handler.Auth)
	s.webServer = &http.Server{
		Addr:        ":8080",
		Handler:     sm,
		BaseContext: func(_ net.Listener) context.Context { return s.ctx },
	}
	s.mailList = make(chan *gmail.Message)
	log.Println("Starting server...")
	go s.ServeWeb()
	google_auth.StartAuth(authConfig)
	s.client_cg = contact_generator.NewContactGenerator()
	s.client_mr = mail_reciever.NewMailReciever(option.WithHTTPClient(google_auth.GetClient(authConfig)), *authConfig)
	log.Println("Starting listener...")
	go s.ListenForEmails()
	log.Println("Authorization completed")
	log.Println("Starting main loop...")
	s.Run()
}
func (s *Server) Close() {
	s.client_cg.Close()
}
func (s *Server) ServeWeb() {
	fmt.Println("Server started on :8080")
	if err := s.webServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		s.errChan <- err
	}
	close(s.errChan)
}
func (s *Server) ListenForEmails() {
	pollInterval := 10 * time.Second
	if err := s.client_mr.ListenForEmails(s.ctx, pollInterval, s.mailList); err != nil {
		s.errChan <- err
	}
}
func (s *Server) HandleEmail(mail *gmail.Message) {
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
	emails, err := google_auth.GetEmails()
	if err != nil {
		log.Printf("Error getting emails: %v", err)
		return
	}
	if !slices.Contains(emails, sender) {
		log.Printf("Email not from sender: %s, from: %s", emails, sender)
		return
	}
	authConfig := google_auth.AuthConfig{Email: sender, Scopes: []string{people.ContactsScope}}
	user_auth := option.WithHTTPClient(google_auth.GetClient(&authConfig))
	client_ca := contact_adder.NewContactAdder(user_auth)

	mailContent, err := s.client_mr.GetMessage(mail.Id)
	if err != nil {
		log.Printf("Error getting message: %v", err)
		return
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
	contact := s.client_cg.Generate(fullMailText)
	log.Println(contact)
	client_ca.AddContact(contact)
	s.client_mr.Reply(mailContent.Id, contact)
}

func (s *Server) Run() {
	for {
		select {
		case mail := <-s.mailList:
			s.HandleEmail(mail)
		case err := <-s.errChan:
			log.Printf("Server error: %v\n", err)
			s.cancel()
			return
		case <-s.ctx.Done():
			log.Println("Shutting down gracefully...")
			shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer shutdownCancel()

			s.cancel()
			if err := s.webServer.Shutdown(shutdownCtx); err != nil {
				log.Printf("Server shutdown error: %v\n", err)
			}
			return
		}

	}
}
