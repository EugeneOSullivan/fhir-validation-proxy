package validator

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

var Profiles = map[string]StructureDefinition{}

type StructureDefinition struct {
	URL      string `json:"url"`
	Snapshot struct {
		Element []ElementDefinition `json:"element"`
	} `json:"snapshot"`
}

type ElementDefinition struct {
	Path string `json:"path"`
	Min  int    `json:"min"`
}

func LoadProfiles(dir string) error {
	return filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".json") {
			return nil
		}

		data, err := ioutil.ReadFile(path)
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
