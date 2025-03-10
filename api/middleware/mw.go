package middleware

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/EO-DataHub/eodhp-workspace-services/internal/authn"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

type contextKey string
type tokenKey string

const ClaimsKey contextKey = "claims"
const TokenKey tokenKey = "token"

// JWTMiddleware parses the JWT token and adds claims to the request context.
func JWTMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			logger := zerolog.Ctx(r.Context()).With().
				Str("handler", "JWTMiddleware").Logger()

			// Get the Authorization header
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				logger.Debug().Msg("authorization header missing")
				http.Error(w, "authorization header missing",
					http.StatusUnauthorized)
				return
			}

			// Check the Authorization header format
			token := strings.TrimPrefix(authHeader, "Bearer ")
			if token == authHeader {
				logger.Error().Msg("invalid token format")
				http.Error(w, "invalid token format", http.StatusUnauthorized)
				return
			}

			// Parse the token for JWT claims
			claims, err := authn.ParseClaims(token)

			if err != nil {
				logger.Error().Err(err).Msg("invalid bearer jwt token")
				http.Error(w, "invalid bearer jwt token", http.StatusUnauthorized)
				return
			}

			// Add the token and claims to the context
			ctx := context.WithValue(r.Context(), TokenKey, token)
			ctx = context.WithValue(ctx, ClaimsKey, claims)

			next.ServeHTTP(w, r.WithContext(ctx))
		},
	)
}

// WithLogger adds a logger to the context and logs request information.
func WithLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			logger := log.With().
				Str("host", r.Host).
				Str("method", r.Method).
				Str("url", r.URL.String()).
				Str("remote_addr", r.RemoteAddr).
				Time("timestamp", time.Now()).
				Logger()

			// Add the logger to the context
			ctx := logger.WithContext(r.Context())
			next.ServeHTTP(w, r.WithContext(ctx))
		},
	)
}
