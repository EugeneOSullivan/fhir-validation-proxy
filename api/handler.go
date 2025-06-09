package api

import (
	"encoding/json"
	"fhir-validation-proxy/internal/validator"
	"io"
	"net/http"
)

func ValidateHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Only POST allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read body", http.StatusBadRequest)
		return
	}

	var resource map[string]interface{}
	if err := json.Unmarshal(body, &resource); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	result := validator.Validate(resource)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}
