package google_auth

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

type TokenWithEmail struct {
	Token *oauth2.Token
	Email string
}

func GetUrl(email string, scopes []string) string {
	b, err := os.ReadFile("credentials.json")
	if err != nil {
		log.Fatalf("Unable to read client secret file: %v", err)
	}

	config, err := google.ConfigFromJSON(b, scopes...)
	if err != nil {
		log.Fatalf("Unable to parse client secret file to config: %v", err)
	}
	opts := []oauth2.AuthCodeOption{
		oauth2.AccessTypeOffline,
		oauth2.ApprovalForce,
	}
	return config.AuthCodeURL(email, opts...)
}

func HandleAuthCode(email string, code string, scopes []string) error {
	b, err := os.ReadFile("credentials.json")
	if err != nil {
		return fmt.Errorf("unable to read client secret file: %v", err)
	}

	config, err := google.ConfigFromJSON(b, scopes...)
	if err != nil {
		return fmt.Errorf("unable to parse client secret file to config: %v", err)
	}

	tok, err := config.Exchange(context.Background(), code)
	if err != nil {
		return fmt.Errorf("unable to retrieve token from web: %v", err)
	}

	SaveToken(email, tok)
	return nil
}
func StartAuth(email string, scopes []string) {
	if _, err := TokenFromFile(email); err != nil {
		url := GetUrl(email, scopes)
		fmt.Println("Please authorize at:", url)
		for {
			if _, err := TokenFromFile(email); err == nil {
				break
			}
			time.Sleep(1 * time.Second)
		}
	}
}

func GetClient(email string, scopes []string) *http.Client {
	b, err := os.ReadFile("credentials.json")
	if err != nil {
		log.Fatalf("Unable to read client secret file: %v", err)
	}

	config, err := google.ConfigFromJSON(b, scopes...)
	if err != nil {
		log.Fatalf("Unable to parse client secret file to config: %v", err)
	}
	tok, err := TokenFromFile(email)
	if err != nil {
		log.Fatalf("Unable to retrieve token from file: %v", err)
	}
	return config.Client(context.Background(), tok)
}

func TokenFromFile(email string) (*oauth2.Token, error) {
	f, err := os.Open("tokens.json")
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var tok []*TokenWithEmail
	err = json.NewDecoder(f).Decode(&tok)
	if err != nil {
		return nil, err
	}
	for _, t := range tok {
		if t.Email == email {
			return t.Token, nil
		}
	}
	return nil, fmt.Errorf("token not found for email: %s", email)
}

func SaveToken(email string, token *oauth2.Token) {
	var tok []*TokenWithEmail

	// Try to read existing tokens
	if f, err := os.Open("tokens.json"); err == nil {
		json.NewDecoder(f).Decode(&tok)
		f.Close()
	}

	// Update or append the token
	for _, t := range tok {
		if t.Email == email {
			t.Token = token
			goto write
		}
	}
	tok = append(tok, &TokenWithEmail{Token: token, Email: email})

	// Write back to file
write:
	f, err := os.OpenFile("tokens.json", os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		log.Fatalf("Unable to cache oauth token: %v", err)
	}
	defer f.Close()

	if err := json.NewEncoder(f).Encode(tok); err != nil {
		log.Fatalf("Unable to encode token to file: %v", err)
	}
}

func GetEmails() ([]string, error) {
	f, err := os.Open("tokens.json")
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var tok []*TokenWithEmail
	err = json.NewDecoder(f).Decode(&tok)
	if err != nil {
		return nil, err
	}
	emails := make([]string, len(tok))
	for i, t := range tok {
		if t.Email == "contacterutil@gmail.com" {
			continue
		}
		emails[i] = t.Email
	}
	return emails, nil
}
