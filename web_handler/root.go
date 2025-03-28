package web_handler

import (
	"MailContactUtilty/google_auth"
	"fmt"
	"net/http"
	"slices"

	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/people/v1"
)

func Register(w http.ResponseWriter, r *http.Request) {
	email := r.URL.Query().Get("email")
	if email == "" {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, "Missing email parameter")
		return
	}
	emails, err := google_auth.GetEmails()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if slices.Contains(emails, email) {
		w.WriteHeader(http.StatusConflict)
		fmt.Fprint(w, "Email already registered")
		return
	}
	url := google_auth.GetUrl(email, []string{people.ContactsScope, gmail.GmailReadonlyScope, gmail.GmailModifyScope})
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, `<a href="`+url+`">Click here to authorize</a>`)
}
func Auth(w http.ResponseWriter, r *http.Request) {
	state := r.URL.Query().Get("state")
	if state == "" {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, "Missing state parameter")
		return
	}
	code := r.URL.Query().Get("code")
	if code == "" {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, "Missing code parameter")
		return
	}

	err := google_auth.HandleAuthCode(state, code, []string{people.ContactsScope, gmail.GmailReadonlyScope, gmail.GmailModifyScope})
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "Error: %v", err)
		return
	}
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "Successfully authorized! You can close this window.")
}
