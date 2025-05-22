package api

import (
	"encoding/json"
	"net/http"
)

type AuthRequest struct {
	Password string `json:"password"`
}

type AuthResponse struct {
	SessionToken string `json:"sessionToken"`
}

func (a *API) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sessionToken := r.Header.Get("Authorization")
		if sessionToken == "" {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		valid, err := a.authController.ValidateSessionToken(r.Context(), sessionToken)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if !valid {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (a *API) postAuthLogin(w http.ResponseWriter, r *http.Request) {
	var request AuthRequest
	err := json.NewDecoder(r.Body).Decode(&request)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	sessionToken, err := a.authController.Authenticate(r.Context(), request.Password)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	response := AuthResponse{
		SessionToken: sessionToken,
	}

	json.NewEncoder(w).Encode(response)
}
