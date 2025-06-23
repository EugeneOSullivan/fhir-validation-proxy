package validator

import (
	"testing"
)

func TestValidateTransactionBundleEnhanced(t *testing.T) {
	// Load test recipes
	if err := LoadRecipes("../../configs/recipes.yaml"); err != nil {
		t.Fatalf("Failed to load recipes: %v", err)
	}

	t.Run("valid transaction bundle with default recipe", func(t *testing.T) {
		bundle := map[string]interface{}{
			"resourceType": "Bundle",
			"type":         "transaction",
			"entry": []interface{}{
				map[string]interface{}{
					"resource": map[string]interface{}{
						"resourceType": "Patient",
						"id":           "patient1",
						"active":       true,
						"identifier": []interface{}{
							map[string]interface{}{
								"system": "https://fhir.nhs.uk/Id/nhs-number",
								"value":  "1234567890",
							},
						},
					},
				},
				map[string]interface{}{
					"resource": map[string]interface{}{
						"resourceType": "Provenance",
						"id":           "prov1",
						"recorded":     "2023-01-01T00:00:00Z",
						"target": []interface{}{
							map[string]interface{}{
								"reference": "Patient/patient1",
							},
						},
					},
				},
			},
		}

		errors := ValidateTransactionBundle(bundle)
		if len(errors) > 0 {
			t.Errorf("Expected no errors, got: %v", errors)
		}
	})

	t.Run("transaction bundle with too many patients", func(t *testing.T) {
		bundle := map[string]interface{}{
			"resourceType": "Bundle",
			"type":         "transaction",
			"entry": []interface{}{
				map[string]interface{}{
					"resource": map[string]interface{}{
						"resourceType": "Patient",
						"id":           "patient1",
					},
				},
				map[string]interface{}{
					"resource": map[string]interface{}{
						"resourceType": "Patient",
						"id":           "patient2",
					},
				},
				map[string]interface{}{
					"resource": map[string]interface{}{
						"resourceType": "Patient",
						"id":           "patient3",
					},
				},
				map[string]interface{}{
					"resource": map[string]interface{}{
						"resourceType": "Patient",
						"id":           "patient4",
					},
				},
				map[string]interface{}{
					"resource": map[string]interface{}{
						"resourceType": "Patient",
						"id":           "patient5",
					},
				},
				map[string]interface{}{
					"resource": map[string]interface{}{
						"resourceType": "Patient",
						"id":           "patient6", // Exceeds max of 5
					},
				},
				map[string]interface{}{
					"resource": map[string]interface{}{
						"resourceType": "Provenance",
						"id":           "prov1",
						"recorded":     "2023-01-01T00:00:00Z",
					},
				},
			},
		}

		errors := ValidateTransactionBundle(bundle)
		found := false
		for _, err := range errors {
			if err == "Too many Patient resources: found 6, maximum 5 allowed" {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected error about too many Patient resources, got: %v", errors)
		}
	})

	t.Run("transaction bundle with forbidden resource", func(t *testing.T) {
		bundle := map[string]interface{}{
			"resourceType": "Bundle",
			"type":         "transaction",
			"entry": []interface{}{
				map[string]interface{}{
					"resource": map[string]interface{}{
						"resourceType": "Patient",
						"id":           "patient1",
					},
				},
				map[string]interface{}{
					"resource": map[string]interface{}{
						"resourceType": "Provenance",
						"id":           "prov1",
						"recorded":     "2023-01-01T00:00:00Z",
					},
				},
				map[string]interface{}{
					"resource": map[string]interface{}{
						"resourceType": "Organization", // Forbidden resource
						"id":           "org1",
						"name":         "Test Org",
					},
				},
			},
		}

		errors := ValidateTransactionBundle(bundle)
		found := false
		for _, err := range errors {
			if err == "Forbidden resource type in bundle: Organization" {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected error about forbidden Organization resource, got: %v", errors)
		}
	})

	t.Run("transaction bundle missing required provenance", func(t *testing.T) {
		bundle := map[string]interface{}{
			"resourceType": "Bundle",
			"type":         "transaction",
			"entry": []interface{}{
				map[string]interface{}{
					"resource": map[string]interface{}{
						"resourceType": "Patient",
						"id":           "patient1",
					},
				},
				// Missing Provenance resource
			},
		}

		errors := ValidateTransactionBundle(bundle)
		found := false
		for _, err := range errors {
			if err == "Missing required Provenance resource in transaction" {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected error about missing Provenance resource, got: %v", errors)
		}
	})
}

func TestValidateMessageBundle(t *testing.T) {
	// Load test recipes
	if err := LoadRecipes("../../configs/recipes.yaml"); err != nil {
		t.Fatalf("Failed to load recipes: %v", err)
	}

	t.Run("valid message bundle", func(t *testing.T) {
		bundle := map[string]interface{}{
			"resourceType": "Bundle",
			"type":         "message",
			"entry": []interface{}{
				map[string]interface{}{
					"resource": map[string]interface{}{
						"resourceType": "MessageHeader",
						"id":           "msg1",
						"eventCoding": map[string]interface{}{
							"system": "http://example.org/events",
							"code":   "patient-created",
						},
						"source": map[string]interface{}{
							"name": "Test System",
						},
						"focus": []interface{}{
							map[string]interface{}{
								"reference": "Patient/patient1",
							},
						},
					},
				},
				map[string]interface{}{
					"resource": map[string]interface{}{
						"resourceType": "Patient",
						"id":           "patient1",
						"active":       true,
					},
				},
			},
		}

		errors := ValidateMessageBundle(bundle)
		if len(errors) > 0 {
			t.Errorf("Expected no errors, got: %v", errors)
		}
	})

	t.Run("message bundle missing MessageHeader", func(t *testing.T) {
		bundle := map[string]interface{}{
			"resourceType": "Bundle",
			"type":         "message",
			"entry": []interface{}{
				map[string]interface{}{
					"resource": map[string]interface{}{
						"resourceType": "Patient",
						"id":           "patient1",
						"active":       true,
					},
				},
			},
		}

		errors := ValidateMessageBundle(bundle)
		found := false
		for _, err := range errors {
			if err == "Missing required MessageHeader resource in message bundle" {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected error about missing MessageHeader, got: %v", errors)
		}
	})

	t.Run("message bundle with incomplete MessageHeader", func(t *testing.T) {
		bundle := map[string]interface{}{
			"resourceType": "Bundle",
			"type":         "message",
			"entry": []interface{}{
				map[string]interface{}{
					"resource": map[string]interface{}{
						"resourceType": "MessageHeader",
						"id":           "msg1",
						// Missing required fields like eventCoding, source, focus
					},
				},
				map[string]interface{}{
					"resource": map[string]interface{}{
						"resourceType": "Patient",
						"id":           "patient1",
						"active":       true,
					},
				},
			},
		}

		errors := ValidateMessageBundle(bundle)
		expectedErrors := []string{
			"Missing required MessageHeader field: eventCoding",
			"Missing required MessageHeader field: source",
			"Missing required MessageHeader field: focus",
		}

		for _, expectedError := range expectedErrors {
			found := false
			for _, err := range errors {
				if err == expectedError {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("Expected error '%s', got: %v", expectedError, errors)
			}
		}
	})

	t.Run("message bundle missing Patient", func(t *testing.T) {
		bundle := map[string]interface{}{
			"resourceType": "Bundle",
			"type":         "message",
			"entry": []interface{}{
				map[string]interface{}{
					"resource": map[string]interface{}{
						"resourceType": "MessageHeader",
						"id":           "msg1",
						"eventCoding": map[string]interface{}{
							"code": "test-event",
						},
						"source": map[string]interface{}{
							"name": "Test System",
						},
						"focus": []interface{}{
							map[string]interface{}{
								"reference": "Patient/patient1",
							},
						},
					},
				},
				// Missing Patient resource
			},
		}

		errors := ValidateMessageBundle(bundle)
		found := false
		for _, err := range errors {
			if err == "Insufficient Patient resources in message: found 0, minimum 1 required" {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected error about missing Patient resource, got: %v", errors)
		}
	})
}

func TestHasMessageHeader(t *testing.T) {
	t.Run("bundle with MessageHeader", func(t *testing.T) {
		entries := []interface{}{
			map[string]interface{}{
				"resource": map[string]interface{}{
					"resourceType": "MessageHeader",
					"id":           "msg1",
				},
			},
			map[string]interface{}{
				"resource": map[string]interface{}{
					"resourceType": "Patient",
					"id":           "patient1",
				},
			},
		}

		if !hasMessageHeader(entries) {
			t.Error("Expected to find MessageHeader")
		}
	})

	t.Run("bundle without MessageHeader", func(t *testing.T) {
		entries := []interface{}{
			map[string]interface{}{
				"resource": map[string]interface{}{
					"resourceType": "Patient",
					"id":           "patient1",
				},
			},
		}

		if hasMessageHeader(entries) {
			t.Error("Expected not to find MessageHeader")
		}
	})

	t.Run("empty bundle", func(t *testing.T) {
		entries := []interface{}{}

		if hasMessageHeader(entries) {
			t.Error("Expected not to find MessageHeader in empty bundle")
		}
	})
}