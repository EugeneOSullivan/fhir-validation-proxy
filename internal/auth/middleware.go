package auth

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"google.golang.org/api/option"
	"google.golang.org/api/healthcare/v1"
)

// Middleware provides authentication and authorization middleware
type Middleware struct {
	healthcareService *healthcare.Service
	requireAuth       bool
}

// NewMiddleware creates a new authentication middleware
func NewMiddleware(serviceAccountKey string, requireAuth bool) (*Middleware, error) {
	middleware := &Middleware{
		requireAuth: requireAuth,
	}

	// Only initialize Google Cloud service if authentication is required
	if requireAuth {
		ctx := context.Background()
		
		var opts []option.ClientOption
		if serviceAccountKey != "" {
			opts = append(opts, option.WithCredentialsFile(serviceAccountKey))
		}
		
		service, err := healthcare.NewService(ctx, opts...)
		if err != nil {
			return nil, fmt.Errorf("failed to create healthcare service: %w", err)
		}
		
		middleware.healthcareService = service
	}

	return middleware, nil
}

// AuthenticateRequest validates the request authentication
func (m *Middleware) AuthenticateRequest(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !m.requireAuth {
			next(w, r)
			return
		}

		// Extract Bearer token from Authorization header
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, "Missing Authorization header", http.StatusUnauthorized)
			return
		}

		if !strings.HasPrefix(authHeader, "Bearer ") {
			http.Error(w, "Invalid Authorization header format", http.StatusUnauthorized)
			return
		}

		token := strings.TrimPrefix(authHeader, "Bearer ")
		if token == "" {
			http.Error(w, "Missing token", http.StatusUnauthorized)
			return
		}

		// For now, we'll do basic token validation
		// In production, you'd validate the JWT token against Google's public keys
		if !m.validateToken(token) {
			http.Error(w, "Invalid token", http.StatusUnauthorized)
			return
		}

		// Add user context to request
		ctx := context.WithValue(r.Context(), "user", extractUserFromToken(token))
		r = r.WithContext(ctx)

		next(w, r)
	}
}

// validateToken validates the provided JWT token
func (m *Middleware) validateToken(token string) bool {
	// TODO: Implement proper JWT token validation
	// This should validate the token against Google's public keys
	// For now, we'll accept any non-empty token
	return token != ""
}

// extractUserFromToken extracts user information from the token
func extractUserFromToken(token string) string {
	// TODO: Implement proper token parsing to extract user info
	// For now, return a placeholder
	return "authenticated-user"
}

// RequireScope middleware ensures the request has the required OAuth2 scope
func (m *Middleware) RequireScope(scope string) func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			// TODO: Implement scope validation
			// For now, we'll allow all requests
			next(w, r)
		}
	}
}

// AuditLog middleware logs all requests for audit purposes
func (m *Middleware) AuditLog(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// TODO: Implement proper audit logging
		// This should log to a structured logging system
		fmt.Printf("AUDIT: %s %s from %s\n", r.Method, r.URL.Path, r.RemoteAddr)
		
		next(w, r)
	}
}