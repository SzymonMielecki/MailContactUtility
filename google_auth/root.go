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
	recieverEmail string
	db            *database.Database
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
		db:            db,
		recieverEmail: "",
	}, nil
}

func (a *Auth) GetUrl(ctx context.Context, authConfig AuthConfig) (string, error) {
	b, err := os.ReadFile(authConfig.Path)
	if err != nil {
		return "", fmt.Errorf("unable to read client secret file: %v", err)
	}

	config, err := google.ConfigFromJSON(b, authConfig.Scopes...)
	if err != nil {
		return "", fmt.Errorf("unable to parse client secret file to config: %v", err)
	}
	opts := []oauth2.AuthCodeOption{
		oauth2.AccessTypeOffline,
		oauth2.ApprovalForce,
	}
	return config.AuthCodeURL(authConfig.Email, opts...), nil
}

func (a *Auth) HandleAuthCode(ctx context.Context, authConfig *AuthConfig, code string) error {
	b, err := os.ReadFile(authConfig.Path)
	if err != nil {
		return fmt.Errorf("unable to read client secret file: %v", err)
	}

	config, err := google.ConfigFromJSON(b, authConfig.Scopes...)
	if err != nil {
		return fmt.Errorf("unable to parse client secret file to config: %v", err)
	}

	tok, err := config.Exchange(ctx, code)
	if err != nil {
		return fmt.Errorf("unable to retrieve token from web: %v", err)
	}

	return a.SaveToken(ctx, authConfig, tok)
}

func (a *Auth) StartAuth(ctx context.Context, authConfig *AuthConfig) {
	a.recieverEmail = authConfig.Email
	if _, err := a.TokenFromDb(ctx, authConfig); err != nil {
		url, err := a.GetUrl(ctx, *authConfig)
		if err != nil {
			log.Fatalf("Unable to get URL: %v", err)
		}
		fmt.Println("Please authorize at:", url)
		for {
			if _, err := a.TokenFromDb(ctx, authConfig); err == nil {
				break
			}
			time.Sleep(1 * time.Second)
		}
	}
}

func (a *Auth) GetHTTPClient(ctx context.Context, authConfig *AuthConfig) (*http.Client, error) {
	b, err := os.ReadFile(authConfig.Path)
	if err != nil {
		return nil, fmt.Errorf("unable to read client secret file: %v", err)
	}

	config, err := google.ConfigFromJSON(b, authConfig.Scopes...)
	if err != nil {
		return nil, fmt.Errorf("unable to parse client secret file to config: %v", err)
	}
	tok, err := a.TokenFromDb(ctx, authConfig)
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve token from file: %v", err)
	}
	return config.Client(ctx, tok), nil
}

func (a *Auth) TokenFromDb(ctx context.Context, authConfig *AuthConfig) (*oauth2.Token, error) {
	token, err := a.db.GetToken(ctx, authConfig.Email)
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

func (a *Auth) SaveToken(ctx context.Context, authConfig *AuthConfig, token *oauth2.Token) error {
	email := authConfig.Email
	found, _ := a.db.GetToken(ctx, email)
	if found == nil {
		return a.db.AddToken(ctx, database.Token{
			Email:        email,
			AccessToken:  token.AccessToken,
			RefreshToken: token.RefreshToken,
			Expiry:       token.Expiry,
			TokenType:    token.TokenType,
		})
	}
	return a.db.UpdateToken(ctx, email, token)
}

func (a *Auth) GetEmails(ctx context.Context) ([]string, error) {
	if a.recieverEmail == "" {
		return nil, fmt.Errorf("reciever email is not set")
	}
	emails, err := a.db.GetEmails(ctx)
	if err != nil {
		return nil, err
	}

	filtered := make([]string, 0, len(emails))
	for _, email := range emails {
		if email != a.recieverEmail {
			filtered = append(filtered, email)
		}
	}

	return filtered, nil
}
