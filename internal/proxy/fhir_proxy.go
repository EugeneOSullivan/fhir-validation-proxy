package proxy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"fhir-validation-proxy/internal/config"
	"fhir-validation-proxy/internal/validator"

	"github.com/gorilla/mux"
)

// FHIRProxy handles FHIR operations and validation
type FHIRProxy struct {
	config     *config.Config
	httpClient *http.Client
	baseURL    string
}

// NewFHIRProxy creates a new FHIR proxy instance
func NewFHIRProxy(cfg *config.Config) *FHIRProxy {
	return &FHIRProxy{
		config: cfg,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		baseURL: cfg.GetFHIRStoreURL(),
	}
}

// SetupRoutes configures all FHIR routes
func (p *FHIRProxy) SetupRoutes(router *mux.Router) {
	// Legacy validation endpoint
	router.HandleFunc("/validate", p.ValidateHandler).Methods("POST")

	// FHIR R4 endpoints
	fhirRouter := router.PathPrefix("/fhir").Subrouter()

	// System operations (must come before resource operations)
	fhirRouter.HandleFunc("/$process-message", p.HandleProcessMessage).Methods("POST")
	fhirRouter.HandleFunc("/metadata", p.HandleCapabilityStatement).Methods("GET")

	// Bundle operations
	fhirRouter.HandleFunc("", p.HandleBundleTransaction).Methods("POST")

	// Resource operations
	fhirRouter.HandleFunc("/{resourceType}", p.HandleResourceList).Methods("GET", "POST")
	fhirRouter.HandleFunc("/{resourceType}/_search", p.HandleResourceSearch).Methods("GET", "POST")
	fhirRouter.HandleFunc("/{resourceType}/{id}", p.HandleResourceInstance).Methods("GET", "PUT", "DELETE")
	fhirRouter.HandleFunc("/{resourceType}/{id}/_history", p.HandleResourceHistory).Methods("GET")
	fhirRouter.HandleFunc("/{resourceType}/{id}/_history/{vid}", p.HandleResourceVersion).Methods("GET")

	// Health check and metrics
	router.HandleFunc("/health", p.HealthCheck).Methods("GET")
	router.HandleFunc("/metrics", p.MetricsHandler).Methods("GET")
}

// ValidateHandler handles the legacy validation endpoint
func (p *FHIRProxy) ValidateHandler(w http.ResponseWriter, r *http.Request) {
	// Enterprise security: Check request size
	if r.ContentLength > validator.MaxRequestSize {
		p.writeOperationOutcome(w, http.StatusRequestEntityTooLarge, "Request too large")
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		p.writeOperationOutcome(w, http.StatusBadRequest, "Failed to read request body")
		return
	}

	var resource map[string]interface{}
	if err := json.Unmarshal(body, &resource); err != nil {
		p.writeOperationOutcome(w, http.StatusBadRequest, "Invalid JSON")
		return
	}

	result := validator.Validate(resource)

	// Add enterprise monitoring headers
	w.Header().Set("X-Validation-Duration", result.Duration.String())
	w.Header().Set("X-Resource-Type", result.ResourceType)

	p.writeValidationResult(w, result)
}

// HandleResourceList handles GET/POST requests to /{resourceType}
func (p *FHIRProxy) HandleResourceList(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	resourceType := vars["resourceType"]

	switch r.Method {
	case "GET":
		p.handleResourceSearch(w, r, resourceType)
	case "POST":
		p.handleResourceCreate(w, r, resourceType)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// HandleResourceInstance handles GET/PUT/DELETE requests to /{resourceType}/{id}
func (p *FHIRProxy) HandleResourceInstance(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	resourceType := vars["resourceType"]
	resourceID := vars["id"]

	switch r.Method {
	case "GET":
		p.handleResourceRead(w, r, resourceType, resourceID)
	case "PUT":
		p.handleResourceUpdate(w, r, resourceType, resourceID)
	case "DELETE":
		p.handleResourceDelete(w, r, resourceType, resourceID)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// HandleResourceSearch handles search operations
func (p *FHIRProxy) HandleResourceSearch(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	resourceType := vars["resourceType"]
	p.handleResourceSearch(w, r, resourceType)
}

// HandleResourceHistory handles resource history
func (p *FHIRProxy) HandleResourceHistory(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	resourceType := vars["resourceType"]
	resourceID := vars["id"]

	if p.baseURL == "" {
		p.writeOperationOutcome(w, http.StatusServiceUnavailable, "FHIR server not configured")
		return
	}

	targetURL := fmt.Sprintf("%s/%s/%s/_history", p.baseURL, resourceType, resourceID)
	p.proxyRequest(w, r, targetURL)
}

// HandleResourceVersion handles specific resource versions
func (p *FHIRProxy) HandleResourceVersion(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	resourceType := vars["resourceType"]
	resourceID := vars["id"]
	versionID := vars["vid"]

	if p.baseURL == "" {
		p.writeOperationOutcome(w, http.StatusServiceUnavailable, "FHIR server not configured")
		return
	}

	targetURL := fmt.Sprintf("%s/%s/%s/_history/%s", p.baseURL, resourceType, resourceID, versionID)
	p.proxyRequest(w, r, targetURL)
}

// HandleBundleTransaction handles bundle transactions
func (p *FHIRProxy) HandleBundleTransaction(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Content-Type") != "application/fhir+json" &&
		!strings.Contains(r.Header.Get("Content-Type"), "application/json") {
		p.writeOperationOutcome(w, http.StatusBadRequest, "Content-Type must be application/fhir+json")
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		p.writeOperationOutcome(w, http.StatusBadRequest, "Failed to read request body")
		return
	}

	var bundle map[string]interface{}
	if err := json.Unmarshal(body, &bundle); err != nil {
		p.writeOperationOutcome(w, http.StatusBadRequest, "Invalid JSON")
		return
	}

	// Validate bundle
	result := validator.Validate(bundle)
	if !result.Valid {
		p.writeValidationResult(w, result)
		return
	}

	// Forward to FHIR server if configured
	if p.baseURL != "" {
		p.proxyRequest(w, r, p.baseURL)
	} else {
		p.writeValidationResult(w, result)
	}
}

// HandleProcessMessage handles FHIR messaging
func (p *FHIRProxy) HandleProcessMessage(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		p.writeOperationOutcome(w, http.StatusBadRequest, "Failed to read request body")
		return
	}

	var message map[string]interface{}
	if err := json.Unmarshal(body, &message); err != nil {
		p.writeOperationOutcome(w, http.StatusBadRequest, "Invalid JSON")
		return
	}

	// Validate message bundle
	if message["resourceType"] != "Bundle" {
		p.writeOperationOutcome(w, http.StatusBadRequest, "Message must be a Bundle")
		return
	}

	bundleType, ok := message["type"].(string)
	if !ok || bundleType != "message" {
		p.writeOperationOutcome(w, http.StatusBadRequest, "Bundle type must be 'message'")
		return
	}

	// Validate message structure
	result := validator.Validate(message)
	if !result.Valid {
		p.writeValidationResult(w, result)
		return
	}

	// Process message (forward to FHIR server or handle internally)
	if p.baseURL != "" {
		targetURL := p.baseURL + "/$process-message"
		p.proxyRequest(w, r, targetURL)
	} else {
		// Return success for validated message
		w.Header().Set("Content-Type", "application/fhir+json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"resourceType": "OperationOutcome",
			"issue": []map[string]interface{}{{
				"severity":    "information",
				"code":        "informational",
				"diagnostics": "Message processed successfully",
			}},
		})
	}
}

// HandleCapabilityStatement returns the server's capability statement
func (p *FHIRProxy) HandleCapabilityStatement(w http.ResponseWriter, r *http.Request) {
	if p.baseURL != "" {
		targetURL := p.baseURL + "/metadata"
		p.proxyRequest(w, r, targetURL)
	} else {
		// Return basic capability statement
		capability := map[string]interface{}{
			"resourceType": "CapabilityStatement",
			"status":       "active",
			"date":         time.Now().Format("2006-01-02"),
			"kind":         "instance",
			"fhirVersion":  "4.0.1",
			"format":       []string{"application/fhir+json"},
			"rest": []map[string]interface{}{{
				"mode": "server",
				"resource": []map[string]interface{}{{
					"type": "Patient",
					"interaction": []map[string]interface{}{
						{"code": "read"},
						{"code": "create"},
						{"code": "update"},
						{"code": "delete"},
						{"code": "search-type"},
					},
				}},
			}},
		}

		w.Header().Set("Content-Type", "application/fhir+json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(capability)
	}
}

// HealthCheck returns the health status of the proxy
func (p *FHIRProxy) HealthCheck(w http.ResponseWriter, r *http.Request) {
	status := map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now().Format(time.RFC3339),
		"version":   "1.0.0",
	}

	if p.baseURL != "" {
		// Check upstream FHIR server health
		resp, err := p.httpClient.Get(p.baseURL + "/metadata")
		if err != nil || resp.StatusCode != http.StatusOK {
			status["status"] = "degraded"
			status["upstream"] = "unavailable"
		} else {
			status["upstream"] = "healthy"
			resp.Body.Close()
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(status)
}

// Helper methods

func (p *FHIRProxy) handleResourceCreate(w http.ResponseWriter, r *http.Request, resourceType string) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		p.writeOperationOutcome(w, http.StatusBadRequest, "Failed to read request body")
		return
	}

	var resource map[string]interface{}
	if err := json.Unmarshal(body, &resource); err != nil {
		p.writeOperationOutcome(w, http.StatusBadRequest, "Invalid JSON")
		return
	}

	// Validate resource type matches URL
	if resource["resourceType"] != resourceType {
		p.writeOperationOutcome(w, http.StatusBadRequest,
			fmt.Sprintf("Resource type mismatch: expected %s, got %s", resourceType, resource["resourceType"]))
		return
	}

	// Validate resource
	result := validator.Validate(resource)
	if !result.Valid {
		p.writeValidationResult(w, result)
		return
	}

	// Forward to FHIR server
	if p.baseURL != "" {
		targetURL := fmt.Sprintf("%s/%s", p.baseURL, resourceType)
		p.proxyRequest(w, r, targetURL)
	} else {
		// Return the validated resource with a generated ID
		resource["id"] = fmt.Sprintf("generated-%d", time.Now().Unix())
		w.Header().Set("Content-Type", "application/fhir+json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(resource)
	}
}

func (p *FHIRProxy) handleResourceRead(w http.ResponseWriter, r *http.Request, resourceType, resourceID string) {
	if p.baseURL == "" {
		p.writeOperationOutcome(w, http.StatusServiceUnavailable, "FHIR server not configured")
		return
	}

	targetURL := fmt.Sprintf("%s/%s/%s", p.baseURL, resourceType, resourceID)
	p.proxyRequest(w, r, targetURL)
}

func (p *FHIRProxy) handleResourceUpdate(w http.ResponseWriter, r *http.Request, resourceType, resourceID string) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		p.writeOperationOutcome(w, http.StatusBadRequest, "Failed to read request body")
		return
	}

	var resource map[string]interface{}
	if err := json.Unmarshal(body, &resource); err != nil {
		p.writeOperationOutcome(w, http.StatusBadRequest, "Invalid JSON")
		return
	}

	// Validate resource
	result := validator.Validate(resource)
	if !result.Valid {
		p.writeValidationResult(w, result)
		return
	}

	// Forward to FHIR server
	if p.baseURL != "" {
		targetURL := fmt.Sprintf("%s/%s/%s", p.baseURL, resourceType, resourceID)
		p.proxyRequest(w, r, targetURL)
	} else {
		// Return the updated resource
		resource["id"] = resourceID
		w.Header().Set("Content-Type", "application/fhir+json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(resource)
	}
}

func (p *FHIRProxy) handleResourceDelete(w http.ResponseWriter, r *http.Request, resourceType, resourceID string) {
	if p.baseURL == "" {
		p.writeOperationOutcome(w, http.StatusServiceUnavailable, "FHIR server not configured")
		return
	}

	targetURL := fmt.Sprintf("%s/%s/%s", p.baseURL, resourceType, resourceID)
	p.proxyRequest(w, r, targetURL)
}

func (p *FHIRProxy) handleResourceSearch(w http.ResponseWriter, r *http.Request, resourceType string) {
	if p.baseURL == "" {
		p.writeOperationOutcome(w, http.StatusServiceUnavailable, "FHIR server not configured")
		return
	}

	targetURL := fmt.Sprintf("%s/%s", p.baseURL, resourceType)
	if r.URL.RawQuery != "" {
		targetURL += "?" + r.URL.RawQuery
	}

	p.proxyRequest(w, r, targetURL)
}

func (p *FHIRProxy) proxyRequest(w http.ResponseWriter, r *http.Request, targetURL string) {
	// Create new request
	var body io.Reader
	if r.Body != nil {
		bodyBytes, err := io.ReadAll(r.Body)
		if err != nil {
			p.writeOperationOutcome(w, http.StatusBadRequest, "Failed to read request body")
			return
		}
		body = bytes.NewReader(bodyBytes)
	}

	req, err := http.NewRequestWithContext(r.Context(), r.Method, targetURL, body)
	if err != nil {
		p.writeOperationOutcome(w, http.StatusInternalServerError, "Failed to create proxy request")
		return
	}

	// Copy headers
	for key, values := range r.Header {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}

	// Execute request
	resp, err := p.httpClient.Do(req)
	if err != nil {
		p.writeOperationOutcome(w, http.StatusBadGateway, "Failed to forward request to FHIR server")
		return
	}
	defer resp.Body.Close()

	// Copy response headers
	for key, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}

	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

func (p *FHIRProxy) writeValidationResult(w http.ResponseWriter, result validator.ValidationResult) {
	w.Header().Set("Content-Type", "application/fhir+json")
	if result.Valid {
		w.WriteHeader(http.StatusOK)
	} else {
		w.WriteHeader(http.StatusBadRequest)
	}
	json.NewEncoder(w).Encode(result.Outcome)
}

func (p *FHIRProxy) writeOperationOutcome(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/fhir+json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"resourceType": "OperationOutcome",
		"issue": []map[string]interface{}{{
			"severity":    "error",
			"code":        "invalid",
			"diagnostics": message,
		}},
	})
}

// MetricsHandler provides validation metrics for monitoring
func (p *FHIRProxy) MetricsHandler(w http.ResponseWriter, r *http.Request) {
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
