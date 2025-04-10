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
	AuthClient      *google_auth.Auth
	MailClient      *mail_reciever.MailReciever
	ContactClient   *contact_generator.ContactGenerator
	WebServer       *http.Server
	ctx             context.Context
	cancel          context.CancelFunc
	errChan         chan error
	mailList        chan *gmail.Message
	projectId       string
	credentailsPath string
}

type ServerConfig struct {
	DatabaseName     string
	DatabaseUser     string
	DatabasePassword string
	DatabaseHost     string
	GeminiApiKey     string
	ProjectId        string
}

func NewServer(config ServerConfig) (*Server, error) {
	dbConfig := database.DatabaseConfig{
		Host:     config.DatabaseHost,
		Password: config.DatabasePassword,
		User:     config.DatabaseUser,
		Database: config.DatabaseName,
	}
	ctx, cancel := context.WithCancel(context.Background())
	auth, err := google_auth.NewAuth(ctx, dbConfig)
	if err != nil {
		cancel()
		return nil, err
	}
	contactClient, err := contact_generator.NewContactGenerator(ctx, config.GeminiApiKey)
	if err != nil {
		cancel()
		return nil, err
	}
	return &Server{
		AuthClient:    auth,
		errChan:       make(chan error, 1),
		ctx:           ctx,
		cancel:        cancel,
		mailList:      make(chan *gmail.Message),
		ContactClient: contactClient,
		projectId:     config.ProjectId,
	}, nil
}
func (s *Server) Start(authConfig *google_auth.AuthConfig) {
	s.credentailsPath = authConfig.Path
	sm := http.NewServeMux()
	sm.Handle("/register", web_handler.Register(s.AuthClient, s.credentailsPath))
	sm.Handle("/auth", web_handler.Auth(s.AuthClient, s.credentailsPath))
	s.WebServer = &http.Server{
		Addr:        ":8080",
		Handler:     sm,
		BaseContext: func(_ net.Listener) context.Context { return s.ctx },
	}
	log.Println("Starting server...")
	go s.ServeWeb()
	s.AuthClient.StartAuth(s.ctx, authConfig)
	client, err := s.AuthClient.GetHTTPClient(s.ctx, authConfig)
	if err != nil {
		log.Printf("Unable to create http client: %v", err)
		s.cancel()
		return
	}
	mailClient, err := mail_reciever.NewMailReciever(s.ctx, option.WithHTTPClient(client), *authConfig, s.projectId)
	if err != nil {
		log.Printf("Unable to create mail client: %v", err)
		s.cancel()
		return
	}
	s.MailClient = mailClient
	log.Println("Starting listener...")
	go s.ListenForEmails()
	log.Println("Authorization completed")
	log.Println("Starting main loop...")
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
func (s *Server) ListenForEmails() {
	if err := s.MailClient.ListenForEmails(s.ctx, s.mailList); err != nil {
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
	emails, err := s.AuthClient.GetEmails(s.ctx)
	if err != nil {
		log.Printf("Error getting emails: %v", err)
		return
	}
	if !slices.Contains(emails, sender) {
		log.Printf("Email not from sender: %s, from: %s", emails, sender)
		return
	}
	authConfig := google_auth.AuthConfig{Email: sender, Scopes: []string{people.ContactsScope}, Path: s.credentailsPath}
	client, err := s.AuthClient.GetHTTPClient(s.ctx, &authConfig)
	if err != nil {
		log.Printf("Unable to create http client: %v", err)
		return
	}
	user_auth := option.WithHTTPClient(client)
	client_ca, err := contact_adder.NewContactAdder(s.ctx, user_auth)
	if err != nil {
		log.Printf("Unable to create contact client: %v", err)
		return
	}

	mailContent, err := s.MailClient.GetMessage(s.ctx, mail.Id)
	if err != nil {
		log.Printf("Error getting message: %v", err)
		return
	}
	fullMailText := ""
	images := []contact_generator.ImageData{}
	for _, part := range mailContent.Payload.Parts {
		if part.MimeType == "text/plain" {
			mailString, err := base64.URLEncoding.DecodeString(part.Body.Data)
			if err != nil {
				log.Printf("Error decoding message: %v", err)
				continue
			}
			fullMailText += string(mailString)
		}
		if part.MimeType == "image/jpeg" || part.MimeType == "image/png" || part.MimeType == "image/svg" {
			body, err := s.MailClient.GetAttachment(s.ctx, mail.Id, part.Body.AttachmentId)
			if err != nil {
				log.Printf("Error getting attachment: %v", err)
				continue
			}
			images = append(images, contact_generator.ImageData{
				Type: part.MimeType,
				Data: []byte(body.Data),
			})
		}
	}
	contact, err := s.ContactClient.Generate(s.ctx, fullMailText, images)
	if err != nil {
		log.Printf("Error generating contact: %v", err)
		return
	}
	_, err = client_ca.AddContact(s.ctx, contact)
	if err != nil {
		log.Printf("Error adding contact: %v", err)
		return
	}
	err = s.MailClient.Reply(s.ctx, mailContent.Id, contact, mailContent, sender)
	if err != nil {
		log.Printf("Error replying to message: %v", err)
	}
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
			if err := s.WebServer.Shutdown(shutdownCtx); err != nil {
				log.Printf("Server shutdown error: %v\n", err)
			}
			return
		}

	}
}
