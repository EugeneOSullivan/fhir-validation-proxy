package validator

import (
	"testing"
)

func TestFieldExists(t *testing.T) {
	resource := map[string]interface{}{
		"resourceType": "Patient",
		"name": []interface{}{
			map[string]interface{}{"family": "Smith"},
		},
	}
	if !fieldExists(resource, "Patient.name.family") {
		t.Error("Expected Patient.name.family to exist")
	}
}

func TestCountField(t *testing.T) {
	resource := map[string]interface{}{
		"resourceType": "Patient",
		"address": []interface{}{
			map[string]interface{}{}, map[string]interface{}{},
		},
	}
	count := countField(resource, "Patient.address")
	if count != 2 {
		t.Errorf("Expected count 2, got %d", count)
	}
}

func TestFieldHasFixedValue(t *testing.T) {
	resource := map[string]interface{}{
		"resourceType": "Patient",
		"active":       true,
	}
	if !fieldHasFixedValue(resource, "Patient.active", true) {
		t.Error("Expected Patient.active to have fixed value true")
	}
}

func TestFieldHasAllowedValue(t *testing.T) {
	resource := map[string]interface{}{
		"resourceType": "Patient",
		"gender":       "male",
	}
	allowed := []interface{}{"male", "female", "other", "unknown"}
	if !fieldHasAllowedValue(resource, "Patient.gender", allowed) {
		t.Error("Expected Patient.gender to be in allowed values")
	}
}

func TestFieldMatchesPattern(t *testing.T) {
	tests := []struct {
		name     string
		resource map[string]interface{}
		path     string
		pattern  string
		want     bool
	}{
		{
			name: "simple postal code match",
			resource: map[string]interface{}{
				"resourceType": "Patient",
				"address": map[string]interface{}{
					"postalCode": "CF10 1AA",
				},
			},
			path:    "Patient.address.postalCode",
			pattern: "^[A-Z]{1,2}[0-9R][0-9A-Z]? ?[0-9][A-Z]{2}$",
			want:    true,
		},
		{
			name: "postal code in array",
			resource: map[string]interface{}{
				"resourceType": "Patient",
				"address": []interface{}{
					map[string]interface{}{
						"postalCode": "CF10 1AA",
					},
					map[string]interface{}{
						"postalCode": "SW1A 1AA",
					},
				},
			},
			path:    "Patient.address.postalCode",
			pattern: "^[A-Z]{1,2}[0-9R][0-9A-Z]? ?[0-9][A-Z]{2}$",
			want:    true,
		},
		{
			name: "invalid postal code",
			resource: map[string]interface{}{
				"resourceType": "Patient",
				"address": map[string]interface{}{
					"postalCode": "INVALID",
				},
			},
			path:    "Patient.address.postalCode",
			pattern: "^[A-Z]{1,2}[0-9R][0-9A-Z]? ?[0-9][A-Z]{2}$",
			want:    false,
		},
		{
			name: "non-string value",
			resource: map[string]interface{}{
				"resourceType": "Patient",
				"address": map[string]interface{}{
					"postalCode": 123,
				},
			},
			path:    "Patient.address.postalCode",
			pattern: "^[A-Z]{1,2}[0-9R][0-9A-Z]? ?[0-9][A-Z]{2}$",
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := fieldMatchesPattern(tt.resource, tt.path, tt.pattern)
			if got != tt.want {
				t.Errorf("fieldMatchesPattern() = %v, want %v", got, tt.want)
			}
		})
	}
}
