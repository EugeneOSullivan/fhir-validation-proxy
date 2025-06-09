// Package validator provides FHIR resource validation logic.
package validator

// internal/validator/validator.go

import (
	"fmt"
	"strings"
)

// ValidationResult represents the result of validating a FHIR resource.
type ValidationResult struct {
	Valid   bool
	Errors  []string
	Outcome map[string]interface{}
}

// Validate validates a FHIR resource and returns a ValidationResult.
func Validate(resource map[string]interface{}) ValidationResult {
	errors := ApplyExtraRules(resource["resourceType"].(string), resource)

	if resource["resourceType"] == "Bundle" && resource["type"] == "transaction" {
		errors = append(errors, ValidateTransactionBundle(resource)...) // new logic
	}

	valid := len(errors) == 0

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

	return ValidationResult{
		Valid:   valid,
		Errors:  errors,
		Outcome: outcome,
	}
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

	recipe, hasRecipe := Recipes["default"]
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
		for _, req := range recipe.RequiredResources {
			if !found[req.ResourceType] {
				errs = append(errs, "Missing required resource in bundle: "+req.ResourceType)
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
