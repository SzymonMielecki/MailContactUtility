package google_auth

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"slices"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

type AuthConfig struct {
	Email  string
	Scopes []string
}

type TokenWithEmail struct {
	Token *oauth2.Token
	Email string
}

func GetUrl(authConfig AuthConfig) string {
	email := authConfig.Email
	scopes := authConfig.Scopes
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

func HandleAuthCode(authConfig *AuthConfig, code string) error {
	scopes := authConfig.Scopes
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

	SaveToken(authConfig, tok)
	return nil
}
func StartAuth(authConfig *AuthConfig) {
	if _, err := TokenFromFile(authConfig); err != nil {
		url := GetUrl(*authConfig)
		fmt.Println("Please authorize at:", url)
		for {
			if _, err := TokenFromFile(authConfig); err == nil {
				break
			}
			time.Sleep(1 * time.Second)
		}
	}
}

func GetClient(authConfig *AuthConfig) *http.Client {
	scopes := authConfig.Scopes
	b, err := os.ReadFile("credentials.json")
	if err != nil {
		log.Fatalf("Unable to read client secret file: %v", err)
	}

	config, err := google.ConfigFromJSON(b, scopes...)
	if err != nil {
		log.Fatalf("Unable to parse client secret file to config: %v", err)
	}
	tok, err := TokenFromFile(authConfig)
	if err != nil {
		log.Fatalf("Unable to retrieve token from file: %v", err)
	}
	return config.Client(context.Background(), tok)
}

func TokenFromFile(authConfig *AuthConfig) (*oauth2.Token, error) {
	email := authConfig.Email
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

func SaveToken(authConfig *AuthConfig, token *oauth2.Token) {
	email := authConfig.Email
	var tok []*TokenWithEmail

	if f, err := os.Open("tokens.json"); err == nil {
		json.NewDecoder(f).Decode(&tok)
		f.Close()
	}

	if slices.Contains(tok, &TokenWithEmail{Email: email}) {
		for _, t := range tok {
			if t.Email == email {
				t.Token = token
				break
			}
		}
	} else {
		tok = append(tok, &TokenWithEmail{Token: token, Email: email})
	}

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
