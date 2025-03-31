package web_handler

import (
	"MailContactUtilty/google_auth"
	"MailContactUtilty/server"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"time"

	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/people/v1"
)

type HandleEmailRequest struct {
	Subscription string `json:"subscription"`
	Message      struct {
		Data        []byte    `json:"data"`
		MessageId   string    `json:"messageId"`
		PublishTime time.Time `json:"publishTime"`
	} `json:"message"`
}

func Register(s *server.Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			RegisterScreen().Render(r.Context(), w)
			return
		}
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			MessageScreen("Method not allowed", "Method not allowed").Render(r.Context(), w)
			return
		}
		email := r.FormValue("email")
		emails, err := s.AuthClient.GetEmails()
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			MessageScreen("Error", fmt.Sprintf("Error: %v", err)).Render(r.Context(), w)
			return
		}
		if slices.Contains(emails, email) {
			w.WriteHeader(http.StatusConflict)
			MessageScreen("Email already registered", "The email address you provided is already registered.").Render(r.Context(), w)
			return
		}
		url := s.AuthClient.GetUrl(google_auth.AuthConfig{Email: email, Scopes: []string{people.ContactsScope, gmail.GmailReadonlyScope, gmail.GmailModifyScope}})
		w.WriteHeader(http.StatusOK)
		MessageScreen("Redirecting to authorization", fmt.Sprintf("Redirecting to authorization: %s", url)).Render(r.Context(), w)
		http.Redirect(w, r, url, http.StatusSeeOther)
	}
}
func Auth(s *server.Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		state := r.URL.Query().Get("state")
		if state == "" {
			w.WriteHeader(http.StatusBadRequest)
			MessageScreen("Missing state parameter", "Missing state parameter").Render(r.Context(), w)
			return
		}
		code := r.URL.Query().Get("code")
		if code == "" {
			w.WriteHeader(http.StatusBadRequest)
			MessageScreen("Missing code parameter", "Missing code parameter").Render(r.Context(), w)
			return
		}

		err := s.AuthClient.HandleAuthCode(&google_auth.AuthConfig{Email: state, Scopes: []string{people.ContactsScope, gmail.GmailReadonlyScope, gmail.GmailModifyScope}}, code)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			MessageScreen("Error", fmt.Sprintf("Error: %v", err)).Render(r.Context(), w)
			return
		}
		w.WriteHeader(http.StatusOK)
		MessageScreen("Registration successful", "You have successfully registered.").Render(r.Context(), w)
	}
}

func HandleEmail(s *server.Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			MessageScreen("Method not allowed", "Method not allowed").Render(r.Context(), w)
			return
		}
		var handleEmailRequest HandleEmailRequest
		if err := json.NewDecoder(r.Body).Decode(&handleEmailRequest); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			MessageScreen("Invalid JSON", fmt.Sprintf("Invalid JSON: %v", err)).Render(r.Context(), w)
			return
		}
		s.
	}
}
