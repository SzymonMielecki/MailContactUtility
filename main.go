package main

import (
	"MailContactUtilty/google_auth"
	"MailContactUtilty/server"
	"log"

	"github.com/joho/godotenv"
	"google.golang.org/api/gmail/v1"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Fatal(err)
	}

	s := server.NewServer()
	s.Start(&google_auth.AuthConfig{
		Email:  "contacterutil@gmail.com",
		Scopes: []string{gmail.GmailReadonlyScope, gmail.GmailModifyScope},
	})
	defer s.Close()
}
