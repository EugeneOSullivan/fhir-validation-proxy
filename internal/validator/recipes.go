package validator

import (
	"os"

	"gopkg.in/yaml.v3"
)

type Recipe struct {
	RequiredResources []struct {
		ResourceType string `yaml:"resourceType"`
	} `yaml:"requiredResources"`

	MustReference []struct {
		Source string `yaml:"source"`
		Target string `yaml:"target"`
	} `yaml:"mustReference"`
}

var Recipes = map[string]Recipe{}

type recipeConfig struct {
	Transaction map[string]Recipe `yaml:"transaction"`
}

func LoadRecipes(path string) error {
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
