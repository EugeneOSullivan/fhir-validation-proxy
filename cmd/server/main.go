// main.go
package main

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"

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
			json.NewEncoder(w).Encode(operationOutcome)
			return
		}

		// If valid, forward to actual FHIR server (if configured)
		fhirURL := os.Getenv("FHIR_SERVER_URL")
		if fhirURL != "" {
			proxyResp, err := http.Post(fhirURL, "application/fhir+json", bytes.NewReader(body))
			if err != nil {
				http.Error(w, "Failed to forward to FHIR server", http.StatusBadGateway)
				return
			}
			defer proxyResp.Body.Close()
			w.Header().Set("Content-Type", proxyResp.Header.Get("Content-Type"))
			w.WriteHeader(proxyResp.StatusCode)
			io.Copy(w, proxyResp.Body)
			return
		}

		// If no FHIR server configured, echo back the valid resource
		w.Header().Set("Content-Type", "application/fhir+json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(resource)
	})

	log.Println("Validator running at http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
