// main.go
package main

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"time"

	"fhir-validation-proxy/internal/validator"
)

func main() {
	// FHIR Profiles
	if err := validator.LoadProfiles("configs/profiles"); err != nil {
		log.Fatalf("Failed to load profiles: %v", err)
	}
	// FHIR Rules
	if err := validator.LoadRules("configs/rules.yaml"); err != nil {
		log.Fatalf("Failed to load rules: %v", err)
	}
	// Bundle Recipes
	if err := validator.LoadRecipes("configs/recipes.yaml"); err != nil {
		log.Fatalf("Failed to load recipes: %v", err)
	}

	http.HandleFunc("/validate", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Only POST allowed", http.StatusMethodNotAllowed)
			return
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Failed to read request body", http.StatusBadRequest)
			return
		}

		var resource map[string]interface{}
		err = json.Unmarshal(body, &resource)
		if err != nil {
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}

		result := validator.Validate(resource)

		if !result.Valid {
			operationOutcome := map[string]interface{}{
				"resourceType": "OperationOutcome",
				"issue":        []map[string]interface{}{},
			}
			for _, errMsg := range result.Errors {
				operationOutcome["issue"] = append(operationOutcome["issue"].([]map[string]interface{}), map[string]interface{}{
					"severity":    "error",
					"code":        "invalid",
					"diagnostics": errMsg,
				})
			}
			w.WriteHeader(http.StatusBadRequest)
			w.Header().Set("Content-Type", "application/fhir+json")
			if err := json.NewEncoder(w).Encode(operationOutcome); err != nil {
				http.Error(w, "Failed to encode response", http.StatusInternalServerError)
			}
			return
		}

		// If valid, forward to actual FHIR server (if configured)
		fhirURL := os.Getenv("FHIR_SERVER_URL")
		if fhirURL != "" {
			parsedURL, err := url.ParseRequestURI(fhirURL)
			if err != nil {
				http.Error(w, "Invalid FHIR_SERVER_URL", http.StatusInternalServerError)
				return
			}
			proxyResp, err := http.Post(parsedURL.String(), "application/fhir+json", bytes.NewReader(body))
			if err != nil {
				http.Error(w, "Failed to forward to FHIR server", http.StatusBadGateway)
				return
			}
			defer func() {
				if cerr := proxyResp.Body.Close(); cerr != nil {
					log.Printf("Failed to close proxy response body: %v", cerr)
				}
			}()
			w.Header().Set("Content-Type", proxyResp.Header.Get("Content-Type"))
			w.WriteHeader(proxyResp.StatusCode)
			if _, err := io.Copy(w, proxyResp.Body); err != nil {
				log.Printf("Failed to copy proxy response body: %v", err)
			}
			return
		}

		// If no FHIR server configured, echo back the valid resource
		w.Header().Set("Content-Type", "application/fhir+json")
		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(resource); err != nil {
			http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		}
	})

	log.Println("Validator running at http://localhost:8080")
	srv := &http.Server{
		Addr:         ":8080",
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
	log.Fatal(srv.ListenAndServe())
}
