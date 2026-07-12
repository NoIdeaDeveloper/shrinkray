package auth

import (
	"log"
	"net/http"
)

// LoginHandlerProvider allows providers to implement login handling.
type LoginHandlerProvider interface {
	HandleLogin(w http.ResponseWriter, r *http.Request) error
}

// CallbackHandler handles auth provider callbacks.
func CallbackHandler(provider Provider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if provider == nil {
			http.NotFound(w, r)
			return
		}
		if err := provider.HandleCallback(w, r); err != nil {
			log.Printf("[auth] Callback error: %v", err)
			http.Error(w, "authentication failed", http.StatusBadRequest)
		}
	}
}

// LoginHandler handles auth login requests.
func LoginHandler(provider Provider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		loginProvider, ok := provider.(LoginHandlerProvider)
		if !ok || provider == nil {
			http.NotFound(w, r)
			return
		}
		if err := loginProvider.HandleLogin(w, r); err != nil {
			log.Printf("[auth] Login error: %v", err)
			http.Error(w, "invalid credentials", http.StatusUnauthorized)
		}
	}
}

// LogoutHandler handles auth logout requests.
func LogoutHandler(provider Provider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logoutProvider, ok := provider.(LogoutHandlerProvider)
		if !ok || provider == nil {
			http.NotFound(w, r)
			return
		}
		if err := logoutProvider.HandleLogout(w, r); err != nil {
			log.Printf("[auth] Logout error: %v", err)
			http.Error(w, "logout failed", http.StatusUnauthorized)
		}
	}
}
