package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMiddleware(t *testing.T) {
	t.Run("create middleware without auth", func(t *testing.T) {
		middleware, err := NewMiddleware("", false)
		if err != nil {
			t.Fatalf("Failed to create middleware: %v", err)
		}

		if middleware == nil {
			t.Error("Expected middleware to be created")
		}
	})

	t.Run("authenticate request - auth disabled", func(t *testing.T) {
		middleware, _ := NewMiddleware("", false)

		handler := middleware.AuthenticateRequest(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		req := httptest.NewRequest("GET", "/test", nil)
		rr := httptest.NewRecorder()

		handler(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", rr.Code)
		}
	})

	t.Run("authenticate request - auth enabled, no header", func(t *testing.T) {
		middleware, err := NewMiddleware("", true)
		if err != nil {
			t.Skip("Skipping test due to missing Google Cloud credentials")
		}

		handler := middleware.AuthenticateRequest(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		req := httptest.NewRequest("GET", "/test", nil)
		rr := httptest.NewRecorder()

		handler(rr, req)

		if rr.Code != http.StatusUnauthorized {
			t.Errorf("Expected status 401, got %d", rr.Code)
		}
	})

	t.Run("authenticate request - auth enabled, invalid header format", func(t *testing.T) {
		middleware, err := NewMiddleware("", true)
		if err != nil {
			t.Skip("Skipping test due to missing Google Cloud credentials")
		}

		handler := middleware.AuthenticateRequest(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("Authorization", "Invalid header")
		rr := httptest.NewRecorder()

		handler(rr, req)

		if rr.Code != http.StatusUnauthorized {
			t.Errorf("Expected status 401, got %d", rr.Code)
		}
	})

	t.Run("authenticate request - auth enabled, empty token", func(t *testing.T) {
		middleware, err := NewMiddleware("", true)
		if err != nil {
			t.Skip("Skipping test due to missing Google Cloud credentials")
		}

		handler := middleware.AuthenticateRequest(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("Authorization", "Bearer ")
		rr := httptest.NewRecorder()

		handler(rr, req)

		if rr.Code != http.StatusUnauthorized {
			t.Errorf("Expected status 401, got %d", rr.Code)
		}
	})

	t.Run("authenticate request - auth enabled, valid token", func(t *testing.T) {
		middleware, err := NewMiddleware("", true)
		if err != nil {
			t.Skip("Skipping test due to missing Google Cloud credentials")
		}

		handler := middleware.AuthenticateRequest(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("Authorization", "Bearer valid-token")
		rr := httptest.NewRecorder()

		handler(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", rr.Code)
		}
	})

	t.Run("audit log middleware", func(t *testing.T) {
		middleware, _ := NewMiddleware("", false)

		handler := middleware.AuditLog(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		req := httptest.NewRequest("GET", "/test", nil)
		rr := httptest.NewRecorder()

		handler(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", rr.Code)
		}
	})

	t.Run("require scope middleware", func(t *testing.T) {
		middleware, _ := NewMiddleware("", false)

		scopeHandler := middleware.RequireScope("https://www.googleapis.com/auth/cloud-healthcare")
		handler := scopeHandler(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		req := httptest.NewRequest("GET", "/test", nil)
		rr := httptest.NewRecorder()

		handler(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", rr.Code)
		}
	})
}