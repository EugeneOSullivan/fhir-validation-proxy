package validator

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Profiles holds loaded FHIR StructureDefinitions by URL.
var Profiles = map[string]StructureDefinition{}

// StructureDefinition represents a FHIR StructureDefinition profile.
type StructureDefinition struct {
	URL      string `json:"url"`
	Snapshot struct {
		Element []ElementDefinition `json:"element"`
	} `json:"snapshot"`
}

// ElementDefinition represents an element in a FHIR StructureDefinition.
type ElementDefinition struct {
	Path string `json:"path"`
	Min  int    `json:"min"`
}

// LoadProfiles loads FHIR StructureDefinitions from a directory.
func LoadProfiles(dir string) error {
	return filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".json") {
			return nil
		}

		// #nosec G304 -- path is controlled by directory walk and file extension check
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		var sd StructureDefinition
		if err := json.Unmarshal(data, &sd); err != nil {
			return fmt.Errorf("error parsing profile %s: %w", path, err)
		}

		if sd.URL != "" {
			Profiles[sd.URL] = sd
		}
		return nil
	})
}
