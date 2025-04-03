package main

import (
	"MailContactUtilty/google_auth"
	"MailContactUtilty/server"
	"log"
	"os"

	"github.com/joho/godotenv"
	"google.golang.org/api/gmail/v1"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Fatal(err)
	}

	s, err := server.NewServer(server.ServerConfig{
		DatabaseName:     os.Getenv("DATABASE_DB"),
		DatabaseUser:     os.Getenv("DATABASE_USER"),
		DatabasePassword: os.Getenv("DATABASE_PASSWORD"),
		DatabaseHost:     os.Getenv("DATABASE_HOST"),
		GeminiApiKey:     os.Getenv("GEMINI_API_KEY"),
		ProjectId:        os.Getenv("PROJECT_ID"),
	})
	if err != nil {
		log.Fatal(err)
	}
	s.Start(&google_auth.AuthConfig{
		Email:  "contacterutil@gmail.com",
		Scopes: []string{gmail.GmailReadonlyScope, gmail.GmailModifyScope},
	})
	defer s.Close()
}
