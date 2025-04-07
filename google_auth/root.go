package google_auth

import (
	"MailContactUtilty/database"
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

type Auth struct {
	db  *database.Database
	ctx context.Context
}

type AuthConfig struct {
	Email  string
	Scopes []string
	Path   string
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
		db:  db,
		ctx: ctx,
	}, nil
}

func (a *Auth) GetUrl(authConfig AuthConfig) (string, error) {
	email := authConfig.Email
	scopes := authConfig.Scopes
	b, err := os.ReadFile(authConfig.Path)
	if err != nil {
		return "", fmt.Errorf("unable to read client secret file: %v", err)
	}

	config, err := google.ConfigFromJSON(b, scopes...)
	if err != nil {
		return "", fmt.Errorf("unable to parse client secret file to config: %v", err)
	}
	opts := []oauth2.AuthCodeOption{
		oauth2.AccessTypeOffline,
		oauth2.ApprovalForce,
	}
	return config.AuthCodeURL(email, opts...), nil
}

func (a *Auth) HandleAuthCode(authConfig *AuthConfig, code string) error {
	scopes := authConfig.Scopes
	b, err := os.ReadFile(authConfig.Path)
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

	a.SaveToken(authConfig, tok)
	return nil
}
func (a *Auth) StartAuth(authConfig *AuthConfig) {
	if _, err := a.TokenFromDb(authConfig); err != nil {
		url, err := a.GetUrl(*authConfig)
		if err != nil {
			log.Fatalf("Unable to get URL: %v", err)
		}
		fmt.Println("Please authorize at:", url)
		for {
			if _, err := a.TokenFromDb(authConfig); err == nil {
				break
			}
			time.Sleep(1 * time.Second)
		}
	}
}

func (a *Auth) GetHTTPClient(authConfig *AuthConfig) (*http.Client, error) {
	scopes := authConfig.Scopes
	b, err := os.ReadFile(authConfig.Path)
	if err != nil {
		return nil, fmt.Errorf("unable to read client secret file: %v", err)
	}

	config, err := google.ConfigFromJSON(b, scopes...)
	if err != nil {
		return nil, fmt.Errorf("unable to parse client secret file to config: %v", err)
	}
	tok, err := a.TokenFromDb(authConfig)
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve token from file: %v", err)
	}
	return config.Client(context.Background(), tok), nil
}

func (a *Auth) TokenFromDb(authConfig *AuthConfig) (*oauth2.Token, error) {
	token, err := a.db.GetToken(authConfig.Email)
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
	found, _ := a.db.GetToken(email)
	if found == nil {
		return a.db.AddToken(database.Token{
			Email:        email,
			AccessToken:  token.AccessToken,
			RefreshToken: token.RefreshToken,
			Expiry:       token.Expiry,
			TokenType:    token.TokenType,
		})
	}
	return a.db.UpdateToken(email, token)
}

func (a *Auth) GetEmails() ([]string, error) {
	emails, err := a.db.GetEmails()
	if err != nil {
		return nil, err
	}
	return emails, nil
}
