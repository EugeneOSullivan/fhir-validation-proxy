package validator

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

var ExtraRules = map[string]map[string]FieldRule{}

type FieldRule struct {
	Min           int           `yaml:"min"`
	Max           int           `yaml:"max"`
	FixedValue    interface{}   `yaml:"fixedValue"`
	AllowedValues []interface{} `yaml:"allowedValues"`
	Pattern       string        `yaml:"pattern"`
	MustSupport   bool          `yaml:"mustSupport"`
}

func LoadRules(filepath string) error {
	data, err := os.ReadFile(filepath)
	if err != nil {
		return err
	}
	return yaml.Unmarshal(data, &ExtraRules)
}

func ApplyExtraRules(resourceType string, resource map[string]interface{}) []string {
	errors := []string{}

	rules, ok := ExtraRules[resourceType]
	if !ok {
		return errors
	}

	for path, rule := range rules {
		if rule.Min > 0 && !fieldExists(resource, resourceType+"."+path) {
			errors = append(errors, fmt.Sprintf("Missing required field (min): %s", path))
		}
		if rule.Max > 0 && countField(resource, resourceType+"."+path) > rule.Max {
			errors = append(errors, fmt.Sprintf("Too many instances of field (max %d): %s", rule.Max, path))
		}
		if rule.FixedValue != nil && !fieldHasFixedValue(resource, resourceType+"."+path, rule.FixedValue) {
			errors = append(errors, fmt.Sprintf("Field %s does not have fixed value %v", path, rule.FixedValue))
		}
		if len(rule.AllowedValues) > 0 && !fieldHasAllowedValue(resource, resourceType+"."+path, rule.AllowedValues) {
			errors = append(errors, fmt.Sprintf("Field %s has disallowed value", path))
		}
		if rule.Pattern != "" && !fieldMatchesPattern(resource, resourceType+"."+path, rule.Pattern) {
			errors = append(errors, fmt.Sprintf("Field %s does not match pattern %s", path, rule.Pattern))
		}
	}

	return errors
}

func fieldExists(resource map[string]interface{}, fullPath string) bool {
	parts := strings.Split(fullPath, ".")
	current := resource

	for i := 1; i < len(parts); i++ {
		part := parts[i]
		val, ok := current[part]
		if !ok {
			return false
		}

		if i == len(parts)-1 {
			switch v := val.(type) {
			case map[string]interface{}:
				return true
			case []interface{}:
				return len(v) > 0
			default:
				return true
			}
		}

		switch v := val.(type) {
		case map[string]interface{}:
			current = v
		case []interface{}:
			if len(v) == 0 {
				return false
			}
			// If there are more parts, check if any element matches the rest of the path
			remainingPath := strings.Join(parts[i+1:], ".")
			for _, item := range v {
				if itemMap, ok := item.(map[string]interface{}); ok {
					if fieldExists(itemMap, parts[0]+"."+remainingPath) {
						return true
					}
				}
			}
			return false
		default:
			return false
		}
	}
	return true
}

func countField(resource map[string]interface{}, fullPath string) int {
	parts := strings.Split(fullPath, ".")
	current := resource

	for i := 1; i < len(parts); i++ {
		part := parts[i]
		val, ok := current[part]
		if !ok {
			return 0
		}

		switch v := val.(type) {
		case []interface{}:
			return len(v)
		case map[string]interface{}:
			current = v
		default:
			return 1
		}
	}
	return 0
}

func fieldHasFixedValue(resource map[string]interface{}, fullPath string, expected interface{}) bool {
	parts := strings.Split(fullPath, ".")
	current := resource

	for i := 1; i < len(parts); i++ {
		part := parts[i]
		val, ok := current[part]
		if !ok {
			return false
		}

		if i == len(parts)-1 {
			return val == expected
		}

		switch v := val.(type) {
		case map[string]interface{}:
			current = v
		default:
			return false
		}
	}
	return false
}

func fieldHasAllowedValue(resource map[string]interface{}, fullPath string, allowed []interface{}) bool {
	parts := strings.Split(fullPath, ".")
	current := resource

	for i := 1; i < len(parts); i++ {
		part := parts[i]
		val, ok := current[part]
		if !ok {
			return false
		}

		if i == len(parts)-1 {
			for _, a := range allowed {
				if val == a {
					return true
				}
			}
			return false
		}

		switch v := val.(type) {
		case map[string]interface{}:
			current = v
		default:
			return false
		}
	}
	return false
}

func fieldMatchesPattern(resource map[string]interface{}, fullPath string, pattern string) bool {
	parts := strings.Split(fullPath, ".")
	current := resource

	for i := 1; i < len(parts); i++ {
		part := parts[i]
		val, ok := current[part]
		if !ok {
			fmt.Printf("Field %s not found at path %s\n", part, strings.Join(parts[:i+1], "."))
			return false
		}

		if i == len(parts)-1 {
			strVal, ok := val.(string)
			if !ok {
				fmt.Printf("Field %s is not a string, got %T\n", part, val)
				return false
			}
			re, err := regexp.Compile(pattern)
			if err != nil {
				fmt.Printf("Invalid pattern %s: %v\n", pattern, err)
				return false
			}
			matches := re.MatchString(strVal)
			fmt.Printf("Testing pattern %s against value %q: %v\n", pattern, strVal, matches)
			return matches
		}

		switch v := val.(type) {
		case map[string]interface{}:
			current = v
		case []interface{}:
			if len(v) == 0 {
				return false
			}
			// If there are more parts, check if any element matches the rest of the path
			remainingPath := strings.Join(parts[i+1:], ".")
			for _, item := range v {
				if itemMap, ok := item.(map[string]interface{}); ok {
					if fieldMatchesPattern(itemMap, parts[0]+"."+remainingPath, pattern) {
						return true
					}
				}
			}
			return false
		default:
			fmt.Printf("Unexpected type %T at path %s\n", v, strings.Join(parts[:i+1], "."))
			return false
		}
	}
	return false
}
