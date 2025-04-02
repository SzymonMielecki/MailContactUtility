package google_auth

import (
	"MailContactUtilty/database"
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

type Auth struct {
	db *database.Database
}

type AuthConfig struct {
	Email  string
	Scopes []string
}

type TokenWithEmail struct {
	Token *oauth2.Token
	Email string
}

func NewAuth(ctx context.Context, config database.DatabaseConfig) (*Auth, error) {
	db, err := database.NewDatabase(ctx, config)
	if err != nil {
		return nil, err
	}
	return &Auth{
		db: db,
	}, nil
}

func (a *Auth) GetUrl(authConfig AuthConfig) (string, error) {
	email := authConfig.Email
	scopes := authConfig.Scopes
	b, err := os.ReadFile("credentials.json")
	if err != nil {
		return "", fmt.Errorf("unable to read client secret file: %v", err)
	}

	config, err := google.ConfigFromJSON(b, scopes...)
	if err != nil {
		return "", err
	}
	opts := []oauth2.AuthCodeOption{
		oauth2.AccessTypeOffline,
		oauth2.ApprovalForce,
	}
	return config.AuthCodeURL(email, opts...), nil
}

func (a *Auth) HandleAuthCode(authConfig *AuthConfig, code string) error {
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

	if err := a.SaveToken(authConfig, tok); err != nil {
		return fmt.Errorf("unable to save token: %v", err)
	}
	return nil
}
func (a *Auth) StartAuth(authConfig *AuthConfig) error {
	if _, err := a.TokenFromDb(authConfig); err != nil {
		url, err := a.GetUrl(*authConfig)
		if err != nil {
			return fmt.Errorf("unable to get authorization URL: %v", err)
		}
		fmt.Println("Please authorize at:", url)
		startTime := time.Now()
		for {
			log.Println("Waiting for authorization...")
			if _, err := a.TokenFromDb(authConfig); err == nil {
				return nil
			}
			if time.Since(startTime) > 30*time.Second {
				fmt.Println("Authorization process timed out.")
				return fmt.Errorf("authorization process timed out")
			}
			time.Sleep(1 * time.Second)
		}
	}
	return nil
}

func (a *Auth) GetClient(authConfig *AuthConfig) *http.Client {
	scopes := authConfig.Scopes
	b, err := os.ReadFile("credentials.json")
	if err != nil {
		log.Fatalf("Unable to read client secret file: %v", err)
	}

	config, err := google.ConfigFromJSON(b, scopes...)
	if err != nil {
		log.Fatalf("Unable to parse client secret file to config: %v", err)
	}
	tok, err := a.TokenFromDb(authConfig)
	if err != nil {
		log.Fatalf("Unable to retrieve token from db: %v", err)
	}
	return config.Client(context.Background(), tok)
}

func (a *Auth) TokenFromDb(authConfig *AuthConfig) (*oauth2.Token, error) {
	scopes_as_json, err := json.Marshal(authConfig.Scopes)
	if err != nil {
		return nil, err
	}
	token, err := a.db.GetToken(authConfig.Email, string(scopes_as_json))
	if err != nil {
		return nil, err
	}
	return &oauth2.Token{
		RefreshToken: token.RefreshToken,
		AccessToken:  token.AccessToken,
		Expiry:       token.Expiry,
		TokenType:    token.TokenType,
	}, nil
}

func (a *Auth) SaveToken(authConfig *AuthConfig, token *oauth2.Token) error {
	email := authConfig.Email

	scopes_as_json, err := json.Marshal(authConfig.Scopes)
	if err != nil {
		log.Fatalf("Unable to marshal scopes: %v", err)
	}
	exists, err := a.db.CheckExistsToken(email, string(scopes_as_json))
	if err != nil {
		log.Fatalf("Unable to check token existence: %v", err)
	}
	if exists {
		return nil
	}
	return a.db.AddToken(database.Token{
		Email:          email,
		AccessToken:    token.AccessToken,
		RefreshToken:   token.RefreshToken,
		Expiry:         token.Expiry,
		TokenType:      token.TokenType,
		Scopes_as_json: string(scopes_as_json),
	})
}

func (a *Auth) GetEmails() ([]string, error) {
	tokens, err := a.db.GetTokens()
	if err != nil {
		return nil, err
	}
	emails := make([]string, len(tokens))
	for i, token := range tokens {
		emails[i] = token.Email
	}
	return emails, nil
}

func (a *Auth) GetEmailsForScopes(scopes []string) ([]string, error) {
	scopes_as_json, err := json.Marshal(scopes)
	if err != nil {
		return nil, err
	}
	tokens, err := a.db.GetTokensForScopes(string(scopes_as_json))
	if err != nil {
		return nil, err
	}
	emails := make([]string, len(tokens))
	for i, token := range tokens {
		emails[i] = token.Email
	}
	return emails, nil
}
