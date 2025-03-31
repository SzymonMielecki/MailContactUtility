package main

import (
	"MailContactUtilty/database"
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

	s, err := server.NewServer(database.DatabaseConfig{
		Host:     os.Getenv("DATABASE_HOST"),
		Password: os.Getenv("DATABASE_PASSWORD"),
		User:     "postgres",
		Database: "tokens",
	}, os.Getenv("PROJECT_ID"))
	if err != nil {
		log.Fatal(err)
	}
	s.Start(&google_auth.AuthConfig{
		Email:  "contacterutil@gmail.com",
		Scopes: []string{gmail.GmailReadonlyScope, gmail.GmailModifyScope},
	})
	defer s.Close()
}
