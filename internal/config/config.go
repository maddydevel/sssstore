package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

type Config struct {
	BindAddr             string `json:"bind_addr"`
	DataDir              string `json:"data_dir"`
	AdminAccessKey       string `json:"admin_access_key"`
	AdminSecretKey       string `json:"admin_secret_key"`
	TLSCertFile          string `json:"tls_cert_file"`
	TLSKeyFile           string `json:"tls_key_file"`
	StrictMode           bool   `json:"strict_mode"`
	AuditLogPath         string `json:"audit_log_path"`
	MultipartMaxAgeHours int    `json:"multipart_max_age_hours"`
	ReplicationMode      string `json:"replication_mode"`
	ReplicationDir       string `json:"replication_dir"`
}

func Default(dataDir string) Config {
	return Config{
		BindAddr:             ":9000",
		DataDir:              dataDir,
		AdminAccessKey:       "sssadmin",
		AdminSecretKey:       "sssadmin-secret-change-me",
		StrictMode:           false,
		AuditLogPath:         filepath.Join(dataDir, "audit.log"),
		MultipartMaxAgeHours: 24,
		ReplicationMode:      "none",
		ReplicationDir:       "",
	}
}

func Load(path string) (Config, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}
	var cfg Config
	if err := json.Unmarshal(b, &cfg); err != nil {
		return Config{}, err
	}
	if cfg.BindAddr == "" || cfg.DataDir == "" {
		return Config{}, errors.New("invalid config: bind_addr and data_dir are required")
	}
	if cfg.AuditLogPath == "" {
		cfg.AuditLogPath = filepath.Join(cfg.DataDir, "audit.log")
	}
	if cfg.MultipartMaxAgeHours <= 0 {
		cfg.MultipartMaxAgeHours = 24
	}
	if cfg.ReplicationMode == "" {
		cfg.ReplicationMode = "none"
	}
	if cfg.StrictMode {
		if cfg.AdminSecretKey == "" || cfg.AdminSecretKey == "sssadmin-secret-change-me" {
			return Config{}, errors.New("invalid config: strict_mode requires non-default admin_secret_key")
		}
		if (cfg.TLSCertFile == "") != (cfg.TLSKeyFile == "") {
			return Config{}, errors.New("invalid config: both tls_cert_file and tls_key_file must be set together")
		}
	}
	return cfg, nil
}

func Init(path, dataDir string) (Config, error) {
	cfg := Default(dataDir)
	if err := os.MkdirAll(filepath.Join(dataDir, "buckets"), 0o755); err != nil {
		return Config{}, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return Config{}, err
	}
	b, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return Config{}, err
	}
	if err := os.WriteFile(path, b, 0o600); err != nil {
		return Config{}, err
	}
	return cfg, nil
}
