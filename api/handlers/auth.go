package handlers

import "net/http"

type tokenRefresher interface {
	GetToken() error
}

func ensureKeycloakToken(w http.ResponseWriter, kc tokenRefresher) bool {
	if err := kc.GetToken(); err != nil {
		http.Error(w, "Authentication failed.", http.StatusInternalServerError)
		return false
	}
	return true
}
