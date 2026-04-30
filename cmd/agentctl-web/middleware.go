package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"time"
)

type contextKey string

const (
	userContextKey  contextKey = "user"
	csrfContextKey  contextKey = "csrf_token"
	reqIDContextKey contextKey = "request_id"
)

// RequestIDMiddleware adds a unique request ID.
func RequestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := generateID()
		ctx := context.WithValue(r.Context(), reqIDContextKey, id)
		w.Header().Set("X-Request-ID", id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// AuditMiddleware logs each request with user and timing.
func AuditMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: 200}
		next.ServeHTTP(sw, r)

		user := "anonymous"
		if u, ok := r.Context().Value(userContextKey).(*UserInfo); ok {
			user = u.Username
		}
		reqID, _ := r.Context().Value(reqIDContextKey).(string)

		slog.Info("request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", sw.status,
			"duration_ms", time.Since(start).Milliseconds(),
			"user", user,
			"request_id", reqID,
			"remote_addr", r.RemoteAddr,
		)
	})
}

// AuthMiddleware verifies the bearer token and injects UserInfo.
func AuthMiddleware(authn *TokenAuthenticator) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip auth for login, healthz, and static
			if r.URL.Path == "/auth/login" || r.URL.Path == "/healthz" || r.URL.Path == "/readyz" ||
				r.URL.Path == "/static/" || hasPrefix(r.URL.Path, "/static/") {
				next.ServeHTTP(w, r)
				return
			}

			token := extractBearerToken(r)
			if token == "" {
				http.Redirect(w, r, "/auth/login", http.StatusSeeOther)
				return
			}

			user, err := authn.Authenticate(r.Context(), token)
			if err != nil {
				// Clear invalid cookie
				http.SetCookie(w, &http.Cookie{
					Name:     "agentctl-token",
					Value:    "",
					Path:     "/",
					MaxAge:   -1,
					HttpOnly: true,
					SameSite: http.SameSiteStrictMode,
				})
				http.Redirect(w, r, "/auth/login?error=invalid_token", http.StatusSeeOther)
				return
			}

			ctx := context.WithValue(r.Context(), userContextKey, user)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// CSRFMiddleware implements double-submit cookie CSRF protection.
func CSRFMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Ensure CSRF cookie exists
		csrfCookie, err := r.Cookie("agentctl-csrf")
		if err != nil || csrfCookie.Value == "" {
			csrfCookie = &http.Cookie{
				Name:     "agentctl-csrf",
				Value:    generateID(),
				Path:     "/",
				HttpOnly: false, // JS must read this
				SameSite: http.SameSiteStrictMode,
				Secure:   r.TLS != nil,
			}
			http.SetCookie(w, csrfCookie)
		}

		ctx := context.WithValue(r.Context(), csrfContextKey, csrfCookie.Value)

		// Validate CSRF on mutating methods
		if r.Method == http.MethodPost || r.Method == http.MethodPut || r.Method == http.MethodDelete {
			formToken := r.FormValue("csrf_token")
			if formToken == "" {
				formToken = r.Header.Get("X-CSRF-Token")
			}
			if formToken == "" || formToken != csrfCookie.Value {
				http.Error(w, "CSRF token mismatch", http.StatusForbidden)
				return
			}
		}

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func hasPrefix(path, prefix string) bool {
	return len(path) >= len(prefix) && path[:len(prefix)] == prefix
}

func generateID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}
