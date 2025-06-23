package config

import (
	"os"
	"testing"
	"time"
)

func TestLoadConfig(t *testing.T) {
	t.Run("load default config", func(t *testing.T) {
		cfg, err := LoadConfig("")
		if err != nil {
			t.Fatalf("Failed to load default config: %v", err)
		}

		if cfg.Server.Port != 8080 {
			t.Errorf("Expected default port 8080, got %d", cfg.Server.Port)
		}

		if cfg.Server.ReadTimeout != 10*time.Second {
			t.Errorf("Expected default read timeout 10s, got %v", cfg.Server.ReadTimeout)
		}

		if !cfg.Validation.StrictMode {
			t.Error("Expected strict mode to be true by default")
		}
	})

	t.Run("override with environment variables", func(t *testing.T) {
		os.Setenv("PORT", "9090")
		os.Setenv("GOOGLE_CLOUD_PROJECT", "test-project")
		os.Setenv("VALIDATION_STRICT_MODE", "false")
		defer func() {
			os.Unsetenv("PORT")
			os.Unsetenv("GOOGLE_CLOUD_PROJECT")
			os.Unsetenv("VALIDATION_STRICT_MODE")
		}()

		cfg, err := LoadConfig("")
		if err != nil {
			t.Fatalf("Failed to load config: %v", err)
		}

		if cfg.Server.Port != 9090 {
			t.Errorf("Expected port 9090, got %d", cfg.Server.Port)
		}

		if cfg.GoogleCloud.ProjectID != "test-project" {
			t.Errorf("Expected project ID 'test-project', got %s", cfg.GoogleCloud.ProjectID)
		}

		if cfg.Validation.StrictMode {
			t.Error("Expected strict mode to be false")
		}
	})
}

func TestGetFHIRStoreURL(t *testing.T) {
	t.Run("use base URL if provided", func(t *testing.T) {
		cfg := &Config{
			GoogleCloud: GoogleCloudConfig{
				BaseURL: "https://custom.fhir.server/fhir",
			},
		}

		url := cfg.GetFHIRStoreURL()
		if url != "https://custom.fhir.server/fhir" {
			t.Errorf("Expected custom base URL, got %s", url)
		}
	})

	t.Run("construct URL from Google Cloud settings", func(t *testing.T) {
		cfg := &Config{
			GoogleCloud: GoogleCloudConfig{
				ProjectID:   "test-project",
				Location:    "us-central1",
				DatasetID:   "test-dataset",
				FHIRStoreID: "test-store",
			},
		}

		url := cfg.GetFHIRStoreURL()
		expected := "https://healthcare.googleapis.com/v1/projects/test-project/locations/us-central1/datasets/test-dataset/fhirStores/test-store/fhir"
		if url != expected {
			t.Errorf("Expected %s, got %s", expected, url)
		}
	})

	t.Run("return empty string if incomplete settings", func(t *testing.T) {
		cfg := &Config{
			GoogleCloud: GoogleCloudConfig{
				ProjectID: "test-project",
				// Missing other required fields
			},
		}

		url := cfg.GetFHIRStoreURL()
		if url != "" {
			t.Errorf("Expected empty string, got %s", url)
		}
	})
}