package validator

import "testing"

func TestValidate_ValidPatient(t *testing.T) {
	resource := map[string]interface{}{
		"resourceType": "Patient",
		"name": []interface{}{
			map[string]interface{}{"family": "Smith"},
		},
	}
	result := Validate(resource)
	if !result.Valid {
		t.Errorf("Expected valid=true, got false: %v", result.Errors)
	}
}

func TestValidate_MissingResourceType(t *testing.T) {
	resource := map[string]interface{}{
		"id": "123",
	}
	result := Validate(resource)
	if result.Valid {
		t.Error("Expected valid=false due to missing resourceType")
	}
}

func TestValidate_MissingPatientName(t *testing.T) {
	resource := map[string]interface{}{
		"resourceType": "Patient",
	}
	result := Validate(resource)
	if result.Valid {
		t.Error("Expected valid=false due to missing Patient.name")
	}
}
