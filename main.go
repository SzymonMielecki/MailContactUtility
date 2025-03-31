package main

import (
	"MailContactUtilty/database"
	"MailContactUtilty/google_auth"
	"MailContactUtilty/server"
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"
	"google.golang.org/api/gmail/v1"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Fatal(err)
	}

	fmt.Println("Creating server...")
	s := server.NewServer(database.DatabaseConfig{
		Host:     os.Getenv("DATABASE_HOST"),
		Password: os.Getenv("DATABASE_PASSWORD"),
		User:     "postgres",
		Name:     "tokens",
	})
	fmt.Println("Starting server...")
	fmt.Println("Creating server...")
	s := server.NewServer(database.DatabaseConfig{
		Host:     os.Getenv("DATABASE_HOST"),
		Password: os.Getenv("DATABASE_PASSWORD"),
		User:     "postgres",
		Database: "tokens",
	})
	fmt.Println("Starting server...")
	s.Start(&google_auth.AuthConfig{
		Email:  "contacterutil@gmail.com",
		Scopes: []string{gmail.GmailReadonlyScope, gmail.GmailModifyScope},
	})
	defer s.Close()
}
