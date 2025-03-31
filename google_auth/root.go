package google_auth

import (
	"MailContactUtilty/database"
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

func NewAuth(ctx context.Context, config database.DatabaseConfig) *Auth {
	return &Auth{
		db: database.NewDatabase(ctx, config),
	}
}

func (a *Auth) GetUrl(authConfig AuthConfig) string {
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

	a.SaveToken(authConfig, tok)
	return nil
}
func (a *Auth) StartAuth(authConfig *AuthConfig) {
	if _, err := a.TokenFromDb(authConfig); err != nil {
		url := a.GetUrl(*authConfig)
		fmt.Println("Please authorize at:", url)
		for {
			if _, err := a.TokenFromDb(authConfig); err == nil {
				break
			}
			time.Sleep(1 * time.Second)
		}
	}
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
		log.Fatalf("Unable to retrieve token from file: %v", err)
	}
	return config.Client(context.Background(), tok)
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

func (a *Auth) SaveToken(authConfig *AuthConfig, token *oauth2.Token) {
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
	if err := a.db.AddToken(database.Token{
		Email:        email,
		AccessToken:  token.AccessToken,
		RefreshToken: token.RefreshToken,
		Expiry:       token.Expiry,
		TokenType:    token.TokenType,
	}); err != nil {
		log.Fatalf("Unable to add token to database: %v", err)
	}
}

func (a *Auth) GetEmails() ([]string, error) {
	emails, err := a.db.GetEmails()
	if err != nil {
		return nil, err
	}
	return emails, nil
}
