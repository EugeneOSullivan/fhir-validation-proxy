package api

import (
	"encoding/json"
	"fhir-validation-proxy/internal/validator"
	"io"
	"net/http"
	"time"
)

// ValidateHandler handles FHIR resource validation requests.
func ValidateHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeOperationOutcome(w, http.StatusMethodNotAllowed, "Only POST allowed")
		return
	}

	// Enterprise security: Check request size
	if r.ContentLength > validator.MaxRequestSize {
		writeOperationOutcome(w, http.StatusRequestEntityTooLarge, "Request too large")
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeOperationOutcome(w, http.StatusBadRequest, "Failed to read request body")
		return
	}

	var resource map[string]interface{}
	err = json.Unmarshal(body, &resource)
	if err != nil {
		writeOperationOutcome(w, http.StatusBadRequest, "Invalid JSON")
		return
	}

	result := validator.Validate(resource)

	w.Header().Set("Content-Type", "application/fhir+json")
	w.Header().Set("X-Validation-Duration", result.Duration.String())
	w.Header().Set("X-Resource-Type", result.ResourceType)

	if result.Valid {
		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(result.Outcome); err != nil {
			http.Error(w, "Failed to encode response", http.StatusInternalServerError)
			return
		}
	} else {
		w.WriteHeader(http.StatusBadRequest)
		if err := json.NewEncoder(w).Encode(result.Outcome); err != nil {
			http.Error(w, "Failed to encode response", http.StatusInternalServerError)
			return
		}
	}
}

// HealthCheckHandler provides health status for load balancers and monitoring
func HealthCheckHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	health := map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"service":   "fhir-validation-proxy",
		"version":   "1.0.0",
	}

	json.NewEncoder(w).Encode(health)
}

// MetricsHandler provides validation metrics for monitoring
func MetricsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	metrics := validator.GetMetrics()
	metricsData := map[string]interface{}{
		"total_requests":      metrics.TotalRequests,
		"valid_requests":      metrics.ValidRequests,
		"invalid_requests":    metrics.InvalidRequests,
		"success_rate":        float64(metrics.ValidRequests) / float64(metrics.TotalRequests) * 100,
		"average_duration_ms": metrics.AverageDuration.Milliseconds(),
		"last_request_time":   metrics.LastRequestTime.Format(time.RFC3339),
		"uptime":              time.Since(metrics.LastRequestTime).String(),
	}

	json.NewEncoder(w).Encode(metricsData)
}

func writeOperationOutcome(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/fhir+json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(map[string]interface{}{
		"resourceType": "OperationOutcome",
		"issue": []map[string]interface{}{{
			"severity":    "error",
			"code":        "invalid",
			"diagnostics": message,
		}},
	}); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}
