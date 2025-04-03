package web_handler

import (
	"MailContactUtilty/google_auth"
	"fmt"
	"net/http"
	"slices"

	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/people/v1"
)

func Register(a *google_auth.Auth, credentialsPath string) http.HandlerFunc {
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
		emails, err := a.GetEmails()
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
		url, err := a.GetUrl(google_auth.AuthConfig{Email: email, Scopes: []string{people.ContactsScope, gmail.GmailReadonlyScope, gmail.GmailModifyScope}, Path: credentialsPath})
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			MessageScreen("Error", fmt.Sprintf("Error: %v", err)).Render(r.Context(), w)
			return
		}
		w.WriteHeader(http.StatusOK)
		RedirectScreen(url).Render(r.Context(), w)
	}
}
func Auth(a *google_auth.Auth, credentialsPath string) http.HandlerFunc {
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

		err := a.HandleAuthCode(&google_auth.AuthConfig{Email: state, Scopes: []string{people.ContactsScope, gmail.GmailReadonlyScope, gmail.GmailModifyScope}, Path: credentialsPath}, code)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			MessageScreen("Error", fmt.Sprintf("Error: %v", err)).Render(r.Context(), w)
			return
		}
		w.WriteHeader(http.StatusOK)
		MessageScreen("Registration successful", "You have successfully registered.").Render(r.Context(), w)
	}
}
