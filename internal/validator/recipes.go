package validator

import (
	"os"

	"gopkg.in/yaml.v3"
)

// Recipe represents a bundle recipe for required resources and references.
type Recipe struct {
	RequiredResources []RequiredResource `yaml:"requiredResources"`
	MustReference     []Reference        `yaml:"mustReference"`
	ForbiddenResources []string          `yaml:"forbiddenResources"`
	ConditionalRules  []ConditionalRule  `yaml:"conditionalRules"`
	DataQuality       []DataQualityRule  `yaml:"dataQuality"`
	MessageValidation []MessageRule      `yaml:"messageValidation"`
}

// RequiredResource represents a required resource in a bundle
type RequiredResource struct {
	ResourceType string `yaml:"resourceType"`
	MinCount     int    `yaml:"minCount"`
	MaxCount     int    `yaml:"maxCount"`
	Validation   string `yaml:"validation"`
}

// Reference represents a required reference between resources
type Reference struct {
	Source string `yaml:"source"`
	Target string `yaml:"target"`
}

// ConditionalRule represents a conditional validation rule
type ConditionalRule struct {
	When    string   `yaml:"when"`
	Require []string `yaml:"require"`
}

// DataQualityRule represents a data quality validation rule
type DataQualityRule struct {
	Field         string        `yaml:"field"`
	Validation    string        `yaml:"validation"`
	AllowedValues []interface{} `yaml:"allowedValues"`
}

// MessageRule represents a FHIR message validation rule
type MessageRule struct {
	Field    string `yaml:"field"`
	Required bool   `yaml:"required"`
}

// Recipes holds loaded recipes by name.
var Recipes = map[string]Recipe{}

type recipeConfig struct {
	Transaction map[string]Recipe `yaml:"transaction"`
	Message     map[string]Recipe `yaml:"message"`
}

// LoadRecipes loads bundle recipes from a YAML file.
func LoadRecipes(path string) error {
	// #nosec G304 -- path is controlled by caller and only YAML files are expected
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	var config recipeConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return err
	}

	for k, v := range config.Transaction {
		Recipes["transaction:"+k] = v
	}
	
	for k, v := range config.Message {
		Recipes["message:"+k] = v
	}

	return nil
}
