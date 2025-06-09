package validator

import (
	"os"

	"gopkg.in/yaml.v3"
)

// Recipe represents a bundle recipe for required resources and references.
type Recipe struct {
	RequiredResources []struct {
		ResourceType string `yaml:"resourceType"`
	} `yaml:"requiredResources"`

	MustReference []struct {
		Source string `yaml:"source"`
		Target string `yaml:"target"`
	} `yaml:"mustReference"`
}

// Recipes holds loaded recipes by name.
var Recipes = map[string]Recipe{}

type recipeConfig struct {
	Transaction map[string]Recipe `yaml:"transaction"`
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
		Recipes[k] = v
	}

	return nil
}
