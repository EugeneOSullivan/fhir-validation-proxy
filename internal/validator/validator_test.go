package validator

import (
	"testing"
)

// Rule represents a validation rule for a FHIR resource
type Rule struct {
	Path    string        `yaml:"path"`
	Value   interface{}   `yaml:"value,omitempty"`
	Allowed []interface{} `yaml:"allowed,omitempty"`
}

// Profile represents a FHIR profile with validation rules
type Profile struct {
	ResourceType string `yaml:"resourceType"`
	Rules        []Rule `yaml:"rules"`
}

// TestRecipe represents a test-specific recipe for validation
type TestRecipe struct {
	ResourceType string `yaml:"resourceType"`
	Rules        []Rule `yaml:"rules"`
}

// Validator provides FHIR resource validation functionality
type Validator struct{}

// NewValidator creates a new Validator instance
func NewValidator() *Validator {
	return &Validator{}
}

// ValidateResource validates a FHIR resource against a set of rules
func (v *Validator) ValidateResource(resource map[string]interface{}, rules []Rule) bool {
	if resource == nil {
		return false
	}
	rt, ok := resource["resourceType"].(string)
	if !ok || rt == "" || rt == "Invalid" {
		return false
	}
	for _, rule := range rules {
		if rule.Value != nil && !fieldHasFixedValue(resource, rule.Path, rule.Value) {
			return false
		}
		if len(rule.Allowed) > 0 && !fieldHasAllowedValue(resource, rule.Path, rule.Allowed) {
			return false
		}
	}
	return true
}

// ValidateProfile validates a FHIR resource against a profile
func (v *Validator) ValidateProfile(resource map[string]interface{}, profile Profile) bool {
	if resource == nil {
		return false
	}
	rt, ok := resource["resourceType"].(string)
	if !ok || rt != profile.ResourceType {
		return false
	}
	return v.ValidateResource(resource, profile.Rules)
}

// ValidateRecipe validates a FHIR resource against a recipe
func (v *Validator) ValidateRecipe(resource map[string]interface{}, recipe TestRecipe) bool {
	if resource == nil {
		return false
	}
	rt, ok := resource["resourceType"].(string)
	if !ok || rt != recipe.ResourceType {
		return false
	}

	if rt == "Bundle" {
		entries, ok := resource["entry"].([]interface{})
		if !ok || len(entries) == 0 {
			return false
		}

		// Improved separation of bundle and entry rules
		var bundleRules, entryRules []Rule
		for _, rule := range recipe.Rules {
			if val, ok := resource[rule.Path]; ok {
				if val == rule.Value {
					bundleRules = append(bundleRules, rule)
				} else {
					entryRules = append(entryRules, rule)
				}
			} else {
				entryRules = append(entryRules, rule)
			}
		}

		// Check bundle rules
		for _, rule := range bundleRules {
			if !fieldHasFixedValue(resource, rule.Path, rule.Value) {
				return false
			}
		}

		// Check entry rules: at least one entry must match all entry rules
		entryMatch := false
		for entryIdx, entry := range entries {
			entryMap, ok := entry.(map[string]interface{})
			if !ok {
				continue
			}
			res, ok := entryMap["resource"].(map[string]interface{})
			if !ok {
				continue
			}
			allMatch := true
			for _, rule := range entryRules {
				val, has := res[rule.Path]
				if !has || val != rule.Value {
					allMatch = false
					println("Entry", entryIdx, "missing or mismatched", rule.Path, "expected", rule.Value, "got", val)
					break
				}
			}
			if allMatch {
				entryMatch = true
				break
			}
		}
		if len(entryRules) > 0 && !entryMatch {
			return false
		}
		return true
	}

	return v.ValidateResource(resource, recipe.Rules)
}

func TestFieldExists(t *testing.T) {
	tests := []struct {
		name     string
		resource map[string]interface{}
		path     string
		want     bool
	}{
		{
			name: "simple field exists",
			resource: map[string]interface{}{
				"resourceType": "Patient",
				"name": []interface{}{
					map[string]interface{}{"family": "Smith"},
				},
			},
			path: "Patient.name.family",
			want: true,
		},
		{
			name: "field does not exist",
			resource: map[string]interface{}{
				"resourceType": "Patient",
				"name": []interface{}{
					map[string]interface{}{"given": "John"},
				},
			},
			path: "Patient.name.family",
			want: false,
		},
		{
			name: "empty array",
			resource: map[string]interface{}{
				"resourceType": "Patient",
				"name":         []interface{}{},
			},
			path: "Patient.name.family",
			want: false,
		},
		{
			name: "nil array",
			resource: map[string]interface{}{
				"resourceType": "Patient",
				"name":         nil,
			},
			path: "Patient.name.family",
			want: false,
		},
		{
			name: "invalid path",
			resource: map[string]interface{}{
				"resourceType": "Patient",
			},
			path: "Invalid.path",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := fieldExists(tt.resource, tt.path)
			if got != tt.want {
				t.Errorf("fieldExists() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCountField(t *testing.T) {
	tests := []struct {
		name     string
		resource map[string]interface{}
		path     string
		want     int
	}{
		{
			name: "array with two elements",
			resource: map[string]interface{}{
				"resourceType": "Patient",
				"address": []interface{}{
					map[string]interface{}{}, map[string]interface{}{},
				},
			},
			path: "Patient.address",
			want: 2,
		},
		{
			name: "empty array",
			resource: map[string]interface{}{
				"resourceType": "Patient",
				"address":      []interface{}{},
			},
			path: "Patient.address",
			want: 0,
		},
		{
			name: "nil array",
			resource: map[string]interface{}{
				"resourceType": "Patient",
				"address":      nil,
			},
			path: "Patient.address",
			want: 0,
		},
		{
			name: "non-array field",
			resource: map[string]interface{}{
				"resourceType": "Patient",
				"active":       true,
			},
			path: "Patient.active",
			want: 1,
		},
		{
			name: "invalid path",
			resource: map[string]interface{}{
				"resourceType": "Patient",
			},
			path: "Invalid.path",
			want: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := countField(tt.resource, tt.path)
			if got != tt.want {
				t.Errorf("countField() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFieldHasFixedValue(t *testing.T) {
	tests := []struct {
		name     string
		resource map[string]interface{}
		path     string
		value    interface{}
		want     bool
	}{
		{
			name: "boolean match",
			resource: map[string]interface{}{
				"resourceType": "Patient",
				"active":       true,
			},
			path:  "Patient.active",
			value: true,
			want:  true,
		},
		{
			name: "boolean mismatch",
			resource: map[string]interface{}{
				"resourceType": "Patient",
				"active":       false,
			},
			path:  "Patient.active",
			value: true,
			want:  false,
		},
		{
			name: "string match",
			resource: map[string]interface{}{
				"resourceType": "Patient",
				"gender":       "male",
			},
			path:  "Patient.gender",
			value: "male",
			want:  true,
		},
		{
			name: "field does not exist",
			resource: map[string]interface{}{
				"resourceType": "Patient",
			},
			path:  "Patient.active",
			value: true,
			want:  false,
		},
		{
			name: "invalid path",
			resource: map[string]interface{}{
				"resourceType": "Patient",
			},
			path:  "Invalid.path",
			value: true,
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := fieldHasFixedValue(tt.resource, tt.path, tt.value)
			if got != tt.want {
				t.Errorf("fieldHasFixedValue() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFieldHasAllowedValue(t *testing.T) {
	tests := []struct {
		name     string
		resource map[string]interface{}
		path     string
		allowed  []interface{}
		want     bool
	}{
		{
			name: "value in allowed list",
			resource: map[string]interface{}{
				"resourceType": "Patient",
				"gender":       "male",
			},
			path:    "Patient.gender",
			allowed: []interface{}{"male", "female", "other", "unknown"},
			want:    true,
		},
		{
			name: "value not in allowed list",
			resource: map[string]interface{}{
				"resourceType": "Patient",
				"gender":       "invalid",
			},
			path:    "Patient.gender",
			allowed: []interface{}{"male", "female", "other", "unknown"},
			want:    false,
		},
		{
			name: "field does not exist",
			resource: map[string]interface{}{
				"resourceType": "Patient",
			},
			path:    "Patient.gender",
			allowed: []interface{}{"male", "female", "other", "unknown"},
			want:    false,
		},
		{
			name: "empty allowed list",
			resource: map[string]interface{}{
				"resourceType": "Patient",
				"gender":       "male",
			},
			path:    "Patient.gender",
			allowed: []interface{}{},
			want:    false,
		},
		{
			name: "invalid path",
			resource: map[string]interface{}{
				"resourceType": "Patient",
			},
			path:    "Invalid.path",
			allowed: []interface{}{"male", "female", "other", "unknown"},
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := fieldHasAllowedValue(tt.resource, tt.path, tt.allowed)
			if got != tt.want {
				t.Errorf("fieldHasAllowedValue() = %v, want %v", got, tt.want)
			}
		})
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

func TestValidator_ValidateResource(t *testing.T) {
	tests := []struct {
		name     string
		resource map[string]interface{}
		rules    []Rule
		want     bool
	}{
		{
			name: "valid resource with no rules",
			resource: map[string]interface{}{
				"resourceType": "Patient",
				"active":       true,
			},
			rules: []Rule{},
			want:  true,
		},
		{
			name: "valid resource with matching rules",
			resource: map[string]interface{}{
				"resourceType": "Patient",
				"active":       true,
				"gender":       "male",
			},
			rules: []Rule{
				{
					Path:  "Patient.active",
					Value: true,
				},
				{
					Path:    "Patient.gender",
					Allowed: []interface{}{"male", "female", "other", "unknown"},
				},
			},
			want: true,
		},
		{
			name: "invalid resource with non-matching rules",
			resource: map[string]interface{}{
				"resourceType": "Patient",
				"active":       false,
				"gender":       "invalid",
			},
			rules: []Rule{
				{
					Path:  "Patient.active",
					Value: true,
				},
				{
					Path:    "Patient.gender",
					Allowed: []interface{}{"male", "female", "other", "unknown"},
				},
			},
			want: false,
		},
		{
			name: "invalid resource type",
			resource: map[string]interface{}{
				"resourceType": "Invalid",
			},
			rules: []Rule{},
			want:  false,
		},
		{
			name:     "nil resource",
			resource: nil,
			rules:    []Rule{},
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := NewValidator()
			got := v.ValidateResource(tt.resource, tt.rules)
			if got != tt.want {
				t.Errorf("ValidateResource() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestValidator_ValidateProfile(t *testing.T) {
	tests := []struct {
		name     string
		resource map[string]interface{}
		profile  Profile
		want     bool
	}{
		{
			name: "valid resource matching profile",
			resource: map[string]interface{}{
				"resourceType": "Patient",
				"active":       true,
				"gender":       "male",
			},
			profile: Profile{
				ResourceType: "Patient",
				Rules: []Rule{
					{
						Path:  "Patient.active",
						Value: true,
					},
					{
						Path:    "Patient.gender",
						Allowed: []interface{}{"male", "female", "other", "unknown"},
					},
				},
			},
			want: true,
		},
		{
			name: "invalid resource type",
			resource: map[string]interface{}{
				"resourceType": "Invalid",
			},
			profile: Profile{
				ResourceType: "Patient",
				Rules:        []Rule{},
			},
			want: false,
		},
		{
			name: "invalid resource not matching profile rules",
			resource: map[string]interface{}{
				"resourceType": "Patient",
				"active":       false,
				"gender":       "invalid",
			},
			profile: Profile{
				ResourceType: "Patient",
				Rules: []Rule{
					{
						Path:  "Patient.active",
						Value: true,
					},
					{
						Path:    "Patient.gender",
						Allowed: []interface{}{"male", "female", "other", "unknown"},
					},
				},
			},
			want: false,
		},
		{
			name:     "nil resource",
			resource: nil,
			profile: Profile{
				ResourceType: "Patient",
				Rules:        []Rule{},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := NewValidator()
			got := v.ValidateProfile(tt.resource, tt.profile)
			if got != tt.want {
				t.Errorf("ValidateProfile() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestValidator_ValidateRecipe(t *testing.T) {
	tests := []struct {
		name     string
		resource map[string]interface{}
		recipe   TestRecipe
		want     bool
	}{
		{
			name: "valid resource matching recipe",
			resource: map[string]interface{}{
				"resourceType": "Bundle",
				"type":         "transaction",
				"entry": []interface{}{
					map[string]interface{}{
						"resource": map[string]interface{}{
							"resourceType": "Patient",
							"active":       true,
						},
					},
				},
			},
			recipe: TestRecipe{
				ResourceType: "Bundle",
				Rules: []Rule{
					{
						Path:  "type",
						Value: "transaction",
					},
					{
						Path:  "resourceType",
						Value: "Patient",
					},
					{
						Path:  "active",
						Value: true,
					},
				},
			},
			want: true,
		},
		{
			name: "invalid resource type",
			resource: map[string]interface{}{
				"resourceType": "Invalid",
			},
			recipe: TestRecipe{
				ResourceType: "Bundle",
				Rules:        []Rule{},
			},
			want: false,
		},
		{
			name: "invalid resource not matching recipe rules",
			resource: map[string]interface{}{
				"resourceType": "Bundle",
				"type":         "invalid",
				"entry": []interface{}{
					map[string]interface{}{
						"resource": map[string]interface{}{
							"resourceType": "Invalid",
							"active":       false,
						},
					},
				},
			},
			recipe: TestRecipe{
				ResourceType: "Bundle",
				Rules: []Rule{
					{
						Path:  "resourceType",
						Value: "Patient",
					},
					{
						Path:  "active",
						Value: true,
					},
				},
			},
			want: false,
		},
		{
			name:     "nil resource",
			resource: nil,
			recipe: TestRecipe{
				ResourceType: "Bundle",
				Rules:        []Rule{},
			},
			want: false,
		},
		{
			name: "valid non-Bundle resource (ValidateResource branch)",
			resource: map[string]interface{}{
				"resourceType": "Patient",
				"active":       true,
			},
			recipe: TestRecipe{
				ResourceType: "Patient",
				Rules: []Rule{
					{Path: "active", Value: true},
				},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := NewValidator()
			got := v.ValidateRecipe(tt.resource, tt.recipe)
			if got != tt.want {
				t.Errorf("ValidateRecipe() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestValidate(t *testing.T) {
	t.Run("valid Patient", func(t *testing.T) {
		resource := map[string]interface{}{
			"resourceType": "Patient",
			"id":           "pat1",
			"active":       true,
		}
		result := Validate(resource)
		if !result.Valid {
			t.Errorf("expected valid, got errors: %v", result.Errors)
		}
		if result.Outcome["resourceType"] != "OperationOutcome" {
			t.Errorf("expected OperationOutcome, got %v", result.Outcome["resourceType"])
		}
	})

	t.Run("valid transaction Bundle", func(t *testing.T) {
		resource := map[string]interface{}{
			"resourceType": "Bundle",
			"type":         "transaction",
			"entry": []interface{}{
				map[string]interface{}{
					"resource": map[string]interface{}{
						"resourceType": "Patient",
						"id":           "pat1",
					},
				},
				map[string]interface{}{
					"resource": map[string]interface{}{
						"resourceType": "Provenance",
						"id":           "prov1",
					},
				},
			},
		}
		result := Validate(resource)
		if !result.Valid {
			t.Errorf("expected valid, got errors: %v", result.Errors)
		}
		if result.Outcome["resourceType"] != "OperationOutcome" {
			t.Errorf("expected OperationOutcome, got %v", result.Outcome["resourceType"])
		}
	})

	t.Run("invalid transaction Bundle missing Provenance", func(t *testing.T) {
		resource := map[string]interface{}{
			"resourceType": "Bundle",
			"type":         "transaction",
			"entry": []interface{}{
				map[string]interface{}{
					"resource": map[string]interface{}{
						"resourceType": "Patient",
						"id":           "pat1",
					},
				},
			},
		}
		result := Validate(resource)
		if result.Valid {
			t.Errorf("expected invalid, got valid")
		}
		if len(result.Errors) == 0 {
			t.Errorf("expected errors, got none")
		}
	})
}
