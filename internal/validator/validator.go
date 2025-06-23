// Package validator provides FHIR resource validation logic.
package validator

// internal/validator/validator.go

import (
	"fmt"
	"strings"
	"time"
)

// Enterprise-scale limits
const (
	MaxRequestSize    = 10 * 1024 * 1024 // 10MB
	MaxBundleEntries  = 1000
	MaxValidationTime = 30 // seconds
)

// Simple metrics for enterprise monitoring
type ValidationMetrics struct {
	TotalRequests   int64
	ValidRequests   int64
	InvalidRequests int64
	AverageDuration time.Duration
	LastRequestTime time.Time
}

var metrics = &ValidationMetrics{}

// GetMetrics returns current validation metrics
func GetMetrics() *ValidationMetrics {
	return metrics
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
		metrics.TotalRequests++
		metrics.LastRequestTime = time.Now()
	}()

	// Enterprise security: Check resource size and limits
	if err := validateResourceLimits(resource); err != nil {
		duration := time.Since(start)
		metrics.InvalidRequests++
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

	if resource["resourceType"] == "Bundle" && resource["type"] == "transaction" {
		errors = append(errors, ValidateTransactionBundle(resource)...) // new logic
	}

	valid := len(errors) == 0
	duration := time.Since(start)

	// Update metrics
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
	if resource["resourceType"] == "Bundle" {
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
		found := map[string]bool{}
		for _, e := range entries {
			if entry, ok := e.(map[string]interface{}); ok {
				if res, ok := entry["resource"].(map[string]interface{}); ok {
					if rt, ok := res["resourceType"].(string); ok {
						found[rt] = true
					}
				}
			}
		}
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

		// Check forbidden resources
		for _, forbidden := range recipe.ForbiddenResources {
			if resourceCounts[forbidden] > 0 {
				errs = append(errs, fmt.Sprintf("Forbidden resource type in bundle: %s", forbidden))
			}
		}

		// MustReference
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
				errs = append(errs, fmt.Sprintf("No %s -> %s reference found", rule.Source, rule.Target))
			}
		}
	}

	allRefs := []string{}
	for _, e := range entries {
		if entry, ok := e.(map[string]interface{}); ok {
			if res, ok := entry["resource"].(map[string]interface{}); ok {
				allRefs = append(allRefs, collectReferences(res)...)
			}
		}
	}

	missing := referencesExist(allRefs, bundle)
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
