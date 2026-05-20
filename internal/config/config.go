package config

import (
	"log"

	"github.com/spf13/viper"
)

type TLSConfig struct {
	Enabled           bool   `mapstructure:"enabled"`
	CertFile          string `mapstructure:"cert_file"`
	KeyFile           string `mapstructure:"key_file"`
	ClientCA          string `mapstructure:"client_ca"`
	RequireClientCert bool   `mapstructure:"require_client_cert"`
}

type Config struct {
	ServerPort     string `mapstructure:"server_port"`
	WorkerPoolSize int    `mapstructure:"worker_pool_size"`
	ModelPath      string `mapstructure:"model_path"`
	Logging        struct {
		Level string `mapstructure:"level"`
	} `mapstructure:"logging"`
	Pipeline []map[string]interface{} `mapstructure:"pipeline"`
	S3       struct {
		Endpoint  string `mapstructure:"endpoint"`
		Bucket    string `mapstructure:"bucket"`
		AccessKey string `mapstructure:"access_key"`
		SecretKey string `mapstructure:"secret_key"`
		Region    string `mapstructure:"region"`
		UseSSL    bool   `mapstructure:"use_ssl"`
	} `mapstructure:"s3"`
	APIKey                string `mapstructure:"api_key"`
	SignatureVerification struct {
		Enabled      bool   `mapstructure:"enabled"`
		PublicKeyPem string `mapstructure:"public_key_pem"`
	} `mapstructure:"signature_verification"`
	TLS TLSConfig `mapstructure:"tls"`
	JWT struct {
		Enabled      bool   `mapstructure:"enabled"`
		Issuer       string `mapstructure:"issuer"`
		Audience     string `mapstructure:"audience"`
		JWKSURL      string `mapstructure:"jwks_url"`
		Secret       string `mapstructure:"secret"`
		Algorithm    string `mapstructure:"algorithm"`
		RequireRole  string `mapstructure:"require_role"`
		RequireScope string `mapstructure:"require_scope"`
	} `mapstructure:"jwt"`
	ExecutionProvider struct {
		Type     string `mapstructure:"type"`
		DeviceID int    `mapstructure:"device_id"`
	} `mapstructure:"execution_provider"`
}

type JWTConfig struct {
	Enabled      bool   `mapstructure:"enabled"`
	Issuer       string `mapstructure:"issuer"`
	Audience     string `mapstructure:"audience"`
	JWKSURL      string `mapstructure:"jwks_url"`
	Secret       string `mapstructure:"secret"`
	Algorithm    string `mapstructure:"algorithm"`
	RequireRole  string `mapstructure:"require_role"`
	RequireScope string `mapstructure:"require_scope"`
}

func Load() *Config {
	v := viper.New()
	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.AddConfigPath("./configs")
	v.AutomaticEnv()
	v.SetEnvPrefix("GOEDGEINFER")

	// Set defaults
	v.SetDefault("server_port", "8080")
	v.SetDefault("worker_pool_size", 4)
	v.SetDefault("model_path", "testdata/test_model.onnx")
	v.SetDefault("logging.level", "info")
	v.SetDefault("execution_provider.type", "cpu")
	v.SetDefault("execution_provider.device_id", 0)

	err := v.ReadInConfig()
	if err != nil {
		log.Printf("Warning: could not read config file: %v (using defaults and env)", err)
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		log.Fatalf("Failed to unmarshal config: %v", err)
	}
	return &cfg
}

var AppConfig = Load()

func init() {
	log.Printf("Loaded configuration: %+v", AppConfig)
}
