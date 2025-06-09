package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestValidateHandler_Valid(t *testing.T) {
	body := `{"resourceType":"Patient", "name":[{"family":"Smith"}]}`
	req := httptest.NewRequest(http.MethodPost, "/validate", strings.NewReader(body))
	rw := httptest.NewRecorder()

	ValidateHandler(rw, req)

	if rw.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK, got %d", rw.Code)
	}

	var res map[string]interface{}
	json.NewDecoder(rw.Body).Decode(&res)
	if valid, _ := res["valid"].(bool); !valid {
		t.Errorf("Expected valid=true, got false")
	}
}

func TestValidateHandler_InvalidJSON(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/validate", strings.NewReader("{invalid json"))
	rw := httptest.NewRecorder()

	ValidateHandler(rw, req)

	if rw.Code != http.StatusBadRequest {
		t.Fatalf("Expected 400 Bad Request, got %d", rw.Code)
	}
}
