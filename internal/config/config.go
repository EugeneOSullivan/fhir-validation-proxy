package config

import (
	"os"
	"strconv"
	"time"

	"gopkg.in/yaml.v3"
)

// Config holds the application configuration
type Config struct {
	Server      ServerConfig      `yaml:"server"`
	GoogleCloud GoogleCloudConfig `yaml:"google_cloud"`
	Validation  ValidationConfig  `yaml:"validation"`
	Security    SecurityConfig    `yaml:"security"`
	Monitoring  MonitoringConfig  `yaml:"monitoring"`
}

// ServerConfig holds HTTP server configuration
type ServerConfig struct {
	Port         int           `yaml:"port"`
	ReadTimeout  time.Duration `yaml:"read_timeout"`
	WriteTimeout time.Duration `yaml:"write_timeout"`
	IdleTimeout  time.Duration `yaml:"idle_timeout"`
}

// GoogleCloudConfig holds Google Cloud specific configuration
type GoogleCloudConfig struct {
	ProjectID        string `yaml:"project_id"`
	Location         string `yaml:"location"`
	DatasetID        string `yaml:"dataset_id"`
	FHIRStoreID      string `yaml:"fhir_store_id"`
	ServiceAccountKey string `yaml:"service_account_key"`
	BaseURL          string `yaml:"base_url"`
}

// ValidationConfig holds validation-specific configuration
type ValidationConfig struct {
	StrictMode        bool   `yaml:"strict_mode"`
	ProfileValidation bool   `yaml:"profile_validation"`
	CustomRulesPath   string `yaml:"custom_rules_path"`
	ProfilesPath      string `yaml:"profiles_path"`
	RecipesPath       string `yaml:"recipes_path"`
}

// SecurityConfig holds security-related configuration
type SecurityConfig struct {
	RequireAuthentication bool `yaml:"require_authentication"`
	AuditLogging         bool `yaml:"audit_logging"`
	EncryptionRequired   bool `yaml:"encryption_required"`
}

// MonitoringConfig holds monitoring configuration
type MonitoringConfig struct {
	EnableMetrics bool `yaml:"enable_metrics"`
	MetricsPort   int  `yaml:"metrics_port"`
}

// LoadConfig loads configuration from file and environment variables
func LoadConfig(configPath string) (*Config, error) {
	config := &Config{
		Server: ServerConfig{
			Port:         8080,
			ReadTimeout:  10 * time.Second,
			WriteTimeout: 10 * time.Second,
			IdleTimeout:  60 * time.Second,
		},
		Validation: ValidationConfig{
			StrictMode:        true,
			ProfileValidation: true,
			CustomRulesPath:   "configs/rules.yaml",
			ProfilesPath:      "configs/profiles",
			RecipesPath:       "configs/recipes.yaml",
		},
		Security: SecurityConfig{
			RequireAuthentication: false,
			AuditLogging:         true,
			EncryptionRequired:   false,
		},
		Monitoring: MonitoringConfig{
			EnableMetrics: true,
			MetricsPort:   9090,
		},
	}

	if configPath != "" {
		data, err := os.ReadFile(configPath)
		if err != nil {
			return nil, err
		}
		if err := yaml.Unmarshal(data, config); err != nil {
			return nil, err
		}
	}

	// Override with environment variables
	config.overrideWithEnv()

	return config, nil
}

func (c *Config) overrideWithEnv() {
	if port := os.Getenv("PORT"); port != "" {
		if p, err := strconv.Atoi(port); err == nil {
			c.Server.Port = p
		}
	}

	if projectID := os.Getenv("GOOGLE_CLOUD_PROJECT"); projectID != "" {
		c.GoogleCloud.ProjectID = projectID
	}

	if location := os.Getenv("GOOGLE_CLOUD_LOCATION"); location != "" {
		c.GoogleCloud.Location = location
	}

	if datasetID := os.Getenv("GOOGLE_CLOUD_DATASET_ID"); datasetID != "" {
		c.GoogleCloud.DatasetID = datasetID
	}

	if fhirStoreID := os.Getenv("GOOGLE_CLOUD_FHIR_STORE_ID"); fhirStoreID != "" {
		c.GoogleCloud.FHIRStoreID = fhirStoreID
	}

	if saKey := os.Getenv("GOOGLE_APPLICATION_CREDENTIALS"); saKey != "" {
		c.GoogleCloud.ServiceAccountKey = saKey
	}

	if baseURL := os.Getenv("FHIR_SERVER_URL"); baseURL != "" {
		c.GoogleCloud.BaseURL = baseURL
	}

	if strictMode := os.Getenv("VALIDATION_STRICT_MODE"); strictMode != "" {
		c.Validation.StrictMode = strictMode == "true"
	}

	if requireAuth := os.Getenv("REQUIRE_AUTHENTICATION"); requireAuth != "" {
		c.Security.RequireAuthentication = requireAuth == "true"
	}
}

// GetFHIRStoreURL returns the full URL for the FHIR store
func (c *Config) GetFHIRStoreURL() string {
	if c.GoogleCloud.BaseURL != "" {
		return c.GoogleCloud.BaseURL
	}
	
	if c.GoogleCloud.ProjectID != "" && c.GoogleCloud.Location != "" && 
	   c.GoogleCloud.DatasetID != "" && c.GoogleCloud.FHIRStoreID != "" {
		return "https://healthcare.googleapis.com/v1/projects/" + 
			c.GoogleCloud.ProjectID + "/locations/" + c.GoogleCloud.Location + 
			"/datasets/" + c.GoogleCloud.DatasetID + "/fhirStores/" + 
			c.GoogleCloud.FHIRStoreID + "/fhir"
	}
	
	return ""
}