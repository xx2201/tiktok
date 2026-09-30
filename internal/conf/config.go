package conf

import (
	"fmt"
	"gopkg.in/yaml.v3"
	"os"
	"time"
)

type Config struct {
	Services    map[string]Service `yaml:"services"`
	Registry    Registry           `yaml:"registry"`
	Database    Database           `yaml:"database"`
	Redis       Redis              `yaml:"redis"`
	RabbitMQ    RabbitMQ           `yaml:"rabbitmq"`
	Media       Media              `yaml:"media"`
	JWTKey      string             `yaml:"jwt_key"`
	MessageKeys MessageKeys        `yaml:"message_keys"`
	HTTP        HTTP               `yaml:"http"`
}
type Service struct {
	Name     string `yaml:"name"`
	Address  string `yaml:"address"`
	Endpoint string `yaml:"endpoint"`
}
type Registry struct {
	Enabled   bool     `yaml:"enabled"`
	Endpoints []string `yaml:"endpoints"`
}
type Database struct {
	DSN      string   `yaml:"dsn"`
	Replicas []string `yaml:"replicas"`
	MaxOpen  int      `yaml:"max_open"`
	MaxIdle  int      `yaml:"max_idle"`
}
type Redis struct {
	Address  string `yaml:"address"`
	Password string `yaml:"password"`
	DB       int    `yaml:"db"`
}
type RabbitMQ struct {
	URL          string        `yaml:"url"`
	SyncInterval time.Duration `yaml:"sync_interval"`
}
type Media struct {
	Endpoint  string            `yaml:"endpoint"`
	AccessKey string            `yaml:"access_key"`
	SecretKey string            `yaml:"secret_key"`
	SSL       bool              `yaml:"ssl"`
	Buckets   map[string]string `yaml:"buckets"`
	Expiry    time.Duration     `yaml:"expiry"`
	MaxBytes  int               `yaml:"max_bytes"`
}
type MessageKeys struct {
	Public  string `yaml:"public"`
	Private string `yaml:"private"`
}
type HTTP struct {
	TLSCert string  `yaml:"tls_cert"`
	TLSKey  string  `yaml:"tls_key"`
	Rate    float64 `yaml:"rate"`
	Burst   int     `yaml:"burst"`
}

func Load(path string) (*Config, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var c Config
	if err := yaml.Unmarshal([]byte(os.ExpandEnv(string(content))), &c); err != nil {
		return nil, err
	}
	if len(c.JWTKey) < 32 {
		return nil, fmt.Errorf("jwt_key must contain at least 32 bytes")
	}
	for _, name := range []string{"api", "user", "video", "comment", "favorite", "relation", "message"} {
		s := c.Services[name]
		if s.Name == "" || s.Address == "" || s.Endpoint == "" {
			return nil, fmt.Errorf("service %s configuration is incomplete", name)
		}
	}
	if c.Registry.Enabled && len(c.Registry.Endpoints) == 0 {
		return nil, fmt.Errorf("registry endpoints required")
	}
	if c.Database.DSN == "" || c.Database.MaxOpen <= 0 || c.Database.MaxIdle < 0 {
		return nil, fmt.Errorf("invalid database configuration")
	}
	if c.Media.MaxBytes <= 0 || c.Media.Expiry <= 0 || c.Media.Expiry > 7*24*time.Hour {
		return nil, fmt.Errorf("invalid media limits")
	}
	if c.HTTP.Rate <= 0 || c.HTTP.Burst <= 0 {
		return nil, fmt.Errorf("invalid HTTP rate limits")
	}
	if (c.HTTP.TLSCert == "") != (c.HTTP.TLSKey == "") {
		return nil, fmt.Errorf("both TLS certificate and key are required")
	}
	if c.RabbitMQ.SyncInterval <= 0 {
		return nil, fmt.Errorf("positive sync interval required")
	}
	return &c, nil
}
