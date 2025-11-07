// Package validator provides FHIR resource validation logic.
package validator

// internal/validator/validator.go

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// Enterprise-scale limits
const (
	MaxRequestSize    = 10 * 1024 * 1024 // 10MB
	MaxBundleEntries  = 1000
	MaxValidationTime = 30 // seconds
	ResourceTypeBundle = "Bundle"
)

// ValidationMetrics provides simple metrics for enterprise monitoring.
type ValidationMetrics struct {
	TotalRequests   int64
	ValidRequests   int64
	InvalidRequests int64
	AverageDuration time.Duration
	LastRequestTime time.Time
	mu              sync.RWMutex
}

var metrics = &ValidationMetrics{}

// GetMetrics returns current validation metrics (thread-safe)
func GetMetrics() *ValidationMetrics {
	metrics.mu.RLock()
	defer metrics.mu.RUnlock()
	// Return a copy to prevent external modification
	return &ValidationMetrics{
		TotalRequests:   metrics.TotalRequests,
		ValidRequests:   metrics.ValidRequests,
		InvalidRequests: metrics.InvalidRequests,
		AverageDuration: metrics.AverageDuration,
		LastRequestTime: metrics.LastRequestTime,
	}
}

// ValidationResult represents the result of validating a FHIR resource.
type ValidationResult struct {
	Valid        bool
	Errors       []string
	Outcome      map[string]interface{}
	Duration     time.Duration
	ResourceType string
}

// Validate validates a FHIR resource and returns a ValidationResult.
func Validate(resource map[string]interface{}) ValidationResult {
	start := time.Now()
	defer func() {
		metrics.mu.Lock()
		metrics.TotalRequests++
		metrics.LastRequestTime = time.Now()
		metrics.mu.Unlock()
	}()

	// Enterprise security: Check resource size and limits
	if err := validateResourceLimits(resource); err != nil {
		duration := time.Since(start)
		metrics.mu.Lock()
		metrics.InvalidRequests++
		metrics.mu.Unlock()
		return ValidationResult{
			Valid:  false,
			Errors: []string{err.Error()},
			Outcome: map[string]interface{}{
				"resourceType": "OperationOutcome",
				"issue": []map[string]interface{}{
					{
						"severity":    "error",
						"code":        "invalid",
						"diagnostics": err.Error(),
					},
				},
			},
			Duration: duration,
		}
	}

	errors := ApplyExtraRules(resource["resourceType"].(string), resource)

	// Validate bundles based on type
	if resource["resourceType"] == ResourceTypeBundle {
		bundleType, ok := resource["type"].(string)
		if ok {
			switch bundleType {
			case "transaction":
				errors = append(errors, ValidateTransactionBundle(resource)...)
			case "message":
				errors = append(errors, ValidateMessageBundle(resource)...)
			case "batch":
				errors = append(errors, ValidateBatchBundle(resource)...)
			default:
				// Generic bundle validation
				errors = append(errors, ValidateGenericBundle(resource)...)
			}
		}
	}

	valid := len(errors) == 0
	duration := time.Since(start)

	// Update metrics (thread-safe)
	metrics.mu.Lock()
	if valid {
		metrics.ValidRequests++
	} else {
		metrics.InvalidRequests++
	}

	// Update average duration
	if metrics.TotalRequests > 0 {
		metrics.AverageDuration = time.Duration((int64(metrics.AverageDuration)*(metrics.TotalRequests-1) + int64(duration)) / metrics.TotalRequests)
	} else {
		metrics.AverageDuration = duration
	}
	metrics.mu.Unlock()

	outcome := map[string]interface{}{
		"resourceType": "OperationOutcome",
		"issue":        []map[string]interface{}{},
	}

	if valid {
		outcome["issue"] = append(outcome["issue"].([]map[string]interface{}), map[string]interface{}{
			"severity":    "information",
			"code":        "informational",
			"diagnostics": "Validation successful",
		})
	} else {
		for _, e := range errors {
			outcome["issue"] = append(outcome["issue"].([]map[string]interface{}), map[string]interface{}{
				"severity":    "error",
				"code":        "invalid",
				"diagnostics": e,
			})
		}
	}

	resourceType := "Unknown"
	if rt, ok := resource["resourceType"].(string); ok {
		resourceType = rt
	}

	return ValidationResult{
		Valid:        valid,
		Errors:       errors,
		Outcome:      outcome,
		Duration:     duration,
		ResourceType: resourceType,
	}
}

// validateResourceLimits checks enterprise-scale limits
func validateResourceLimits(resource map[string]interface{}) error {
	// Check bundle entry limits
	if resource["resourceType"] == ResourceTypeBundle {
		if entries, ok := resource["entry"].([]interface{}); ok {
			if len(entries) > MaxBundleEntries {
				return fmt.Errorf("bundle contains too many entries: %d (max: %d)", len(entries), MaxBundleEntries)
			}
		}
	}

	// Additional enterprise checks can be added here
	// - Resource depth limits
	// - Reference count limits
	// - Custom field limits

	return nil
}

// ValidateTransactionBundle validates a transaction bundle and returns errors.
func ValidateTransactionBundle(bundle map[string]interface{}) []string {
	errs := []string{}

	entries, ok := bundle["entry"].([]interface{})
	if !ok {
		return []string{"Invalid or missing bundle entries"}
	}

	if !hasProvenance(entries) {
		errs = append(errs, "Missing required Provenance resource in transaction")
	}

	recipe, hasRecipe := Recipes["transaction:default"]
	if hasRecipe {
		resourceCounts := countResourceTypes(entries)
		errs = append(errs, validateRequiredResources(recipe, resourceCounts)...)
		errs = append(errs, validateForbiddenResources(recipe, resourceCounts)...)
		errs = append(errs, validateMustReference(recipe, entries)...)
	}

	errs = append(errs, validateReferences(entries, bundle)...)

	return errs
}

// countResourceTypes counts occurrences of each resource type in bundle entries.
func countResourceTypes(entries []interface{}) map[string]int {
	resourceCounts := map[string]int{}
	for _, e := range entries {
		if entry, ok := e.(map[string]interface{}); ok {
			if res, ok := entry["resource"].(map[string]interface{}); ok {
				if rt, ok := res["resourceType"].(string); ok {
					resourceCounts[rt]++
				}
			}
		}
	}
	return resourceCounts
}

// validateRequiredResources validates required resource counts against recipe.
func validateRequiredResources(recipe Recipe, resourceCounts map[string]int) []string {
	errs := []string{}
	for _, req := range recipe.RequiredResources {
		count := resourceCounts[req.ResourceType]
		minCount := req.MinCount
		if minCount == 0 {
			minCount = 1 // Default minimum is 1
		}

		if count < minCount {
			errs = append(errs, fmt.Sprintf("Insufficient %s resources: found %d, minimum %d required",
				req.ResourceType, count, minCount))
		}

		if req.MaxCount > 0 && count > req.MaxCount {
			errs = append(errs, fmt.Sprintf("Too many %s resources: found %d, maximum %d allowed",
				req.ResourceType, count, req.MaxCount))
		}
	}
	return errs
}

// validateForbiddenResources validates that forbidden resources are not present.
func validateForbiddenResources(recipe Recipe, resourceCounts map[string]int) []string {
	errs := []string{}
	for _, forbidden := range recipe.ForbiddenResources {
		if resourceCounts[forbidden] > 0 {
			errs = append(errs, fmt.Sprintf("Forbidden resource type in bundle: %s", forbidden))
		}
	}
	return errs
}

// validateMustReference validates that required references exist.
func validateMustReference(recipe Recipe, entries []interface{}) []string {
	errs := []string{}
	resourceMap := buildResourceMap(entries)
	for _, rule := range recipe.MustReference {
		valid := false
		for _, src := range resourceMap[rule.Source] {
			refs := collectReferences(src)
			for _, r := range refs {
				if strings.HasPrefix(r, rule.Target+"/") {
					valid = true
					break
				}
			}
		}
		if !valid {
			errs = append(errs, fmt.Sprintf("No %s -> %s reference found", rule.Source, rule.Target))
		}
	}
	return errs
}

// buildResourceMap builds a map of resource types to their resources.
func buildResourceMap(entries []interface{}) map[string][]map[string]interface{} {
	resourceMap := map[string][]map[string]interface{}{}
	for _, e := range entries {
		if entry, ok := e.(map[string]interface{}); ok {
			if res, ok := entry["resource"].(map[string]interface{}); ok {
				if rt, ok := res["resourceType"].(string); ok {
					resourceMap[rt] = append(resourceMap[rt], res)
				}
			}
		}
	}
	return resourceMap
}

// validateReferences validates that all references in entries exist in the bundle.
func validateReferences(entries []interface{}, bundle map[string]interface{}) []string {
	allRefs := []string{}
	for _, e := range entries {
		if entry, ok := e.(map[string]interface{}); ok {
			if res, ok := entry["resource"].(map[string]interface{}); ok {
				allRefs = append(allRefs, collectReferences(res)...)
			}
		}
	}

	missing := referencesExist(allRefs, bundle)
	errs := []string{}
	for _, ref := range missing {
		errs = append(errs, "Unresolved reference: "+ref)
	}
	return errs
}

func hasProvenance(entries []interface{}) bool {
	for _, e := range entries {
		if entry, ok := e.(map[string]interface{}); ok {
			if res, ok := entry["resource"].(map[string]interface{}); ok {
				if res["resourceType"] == "Provenance" {
					return true
				}
			}
		}
	}
	return false
}

// ValidateBatchBundle validates a batch bundle
func ValidateBatchBundle(bundle map[string]interface{}) []string {
	errs := []string{}
	
	entries, ok := bundle["entry"].([]interface{})
	if !ok {
		return []string{"Invalid or missing bundle entries"}
	}
	
	// Batch bundles have different validation rules than transaction bundles
	// They don't require Provenance, but may have other requirements
	recipe, hasRecipe := Recipes["batch:default"]
	if hasRecipe {
		resourceCounts := map[string]int{}
		for _, e := range entries {
			if entry, ok := e.(map[string]interface{}); ok {
				if res, ok := entry["resource"].(map[string]interface{}); ok {
					if rt, ok := res["resourceType"].(string); ok {
						resourceCounts[rt]++
					}
				}
			}
		}
		
		for _, req := range recipe.RequiredResources {
			count := resourceCounts[req.ResourceType]
			minCount := req.MinCount
			if minCount == 0 {
				minCount = 1
			}
			
			if count < minCount {
				errs = append(errs, fmt.Sprintf("Insufficient %s resources in batch: found %d, minimum %d required",
					req.ResourceType, count, minCount))
			}
			
			if req.MaxCount > 0 && count > req.MaxCount {
				errs = append(errs, fmt.Sprintf("Too many %s resources in batch: found %d, maximum %d allowed",
					req.ResourceType, count, req.MaxCount))
			}
		}
	}
	
	return errs
}

// ValidateGenericBundle validates a generic bundle (no specific type)
func ValidateGenericBundle(bundle map[string]interface{}) []string {
	errs := []string{}
	
	entries, ok := bundle["entry"].([]interface{})
	if !ok {
		return []string{"Invalid or missing bundle entries"}
	}
	
	if len(entries) == 0 {
		errs = append(errs, "Bundle must contain at least one entry")
	}
	
	return errs
}

func collectReferences(resource map[string]interface{}) []string {
	refs := []string{}

	var findRefs func(interface{})
	findRefs = func(data interface{}) {
		switch v := data.(type) {
		case map[string]interface{}:
			for k, val := range v {
				if k == "reference" {
					if s, ok := val.(string); ok {
						refs = append(refs, s)
					}
				} else {
					findRefs(val)
				}
			}
		case []interface{}:
			for _, item := range v {
				findRefs(item)
			}
		}
	}
	findRefs(resource)
	return refs
}

func referencesExist(refs []string, bundle map[string]interface{}) []string {
	missing := []string{}
	seen := map[string]bool{}
	for _, e := range bundle["entry"].([]interface{}) {
		if entry, ok := e.(map[string]interface{}); ok {
			if res, ok := entry["resource"].(map[string]interface{}); ok {
				rt := res["resourceType"].(string)
				id := res["id"].(string)
				seen[rt+"/"+id] = true
			}
		}
	}
	for _, ref := range refs {
		if !seen[ref] {
			missing = append(missing, ref)
		}
	}
	return missing
}

// ValidateMessageBundle validates a message bundle
func ValidateMessageBundle(bundle map[string]interface{}) []string {
	errs := []string{}

	entries, ok := bundle["entry"].([]interface{})
	if !ok {
		return []string{"Invalid or missing bundle entries"}
	}

	// Check for MessageHeader
	if !hasMessageHeader(entries) {
		errs = append(errs, "Missing required MessageHeader resource in message bundle")
	}

	recipe, hasRecipe := Recipes["message:default"]
	if hasRecipe {
		resourceCounts := map[string]int{}
		for _, e := range entries {
			if entry, ok := e.(map[string]interface{}); ok {
				if res, ok := entry["resource"].(map[string]interface{}); ok {
					if rt, ok := res["resourceType"].(string); ok {
						resourceCounts[rt]++
					}
				}
			}
		}

		for _, req := range recipe.RequiredResources {
			count := resourceCounts[req.ResourceType]

			minCount := req.MinCount
			if minCount == 0 {
				minCount = 1
			}

			if count < minCount {
				errs = append(errs, fmt.Sprintf("Insufficient %s resources in message: found %d, minimum %d required",
					req.ResourceType, count, minCount))
			}

			if req.MaxCount > 0 && count > req.MaxCount {
				errs = append(errs, fmt.Sprintf("Too many %s resources in message: found %d, maximum %d allowed",
					req.ResourceType, count, req.MaxCount))
			}
		}

		// Validate message-specific rules
		for _, e := range entries {
			if entry, ok := e.(map[string]interface{}); ok {
				if res, ok := entry["resource"].(map[string]interface{}); ok {
					if res["resourceType"] == "MessageHeader" {
						for _, rule := range recipe.MessageValidation {
							if rule.Required {
								// Check if field exists directly on the resource
								if _, ok := res[rule.Field]; !ok {
									errs = append(errs, fmt.Sprintf("Missing required MessageHeader field: %s", rule.Field))
								}
							}
						}
					}
				}
			}
		}

		// Check references
		resourceMap := map[string][]map[string]interface{}{}
		for _, e := range entries {
			if entry, ok := e.(map[string]interface{}); ok {
				if res, ok := entry["resource"].(map[string]interface{}); ok {
					if rt, ok := res["resourceType"].(string); ok {
						resourceMap[rt] = append(resourceMap[rt], res)
					}
				}
			}
		}

		for _, rule := range recipe.MustReference {
			valid := false
			for _, src := range resourceMap[rule.Source] {
				refs := collectReferences(src)
				for _, r := range refs {
					if strings.HasPrefix(r, rule.Target+"/") {
						valid = true
						break
					}
				}
			}
			if !valid {
				errs = append(errs, fmt.Sprintf("No %s -> %s reference found in message", rule.Source, rule.Target))
			}
		}
	}

	return errs
}

func hasMessageHeader(entries []interface{}) bool {
	for _, e := range entries {
		if entry, ok := e.(map[string]interface{}); ok {
			if res, ok := entry["resource"].(map[string]interface{}); ok {
				if res["resourceType"] == "MessageHeader" {
					return true
				}
			}
		}
	}
	return false
}
