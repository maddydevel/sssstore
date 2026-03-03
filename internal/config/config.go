package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

type Config struct {
	BindAddr       string `json:"bind_addr"`
	DataDir        string `json:"data_dir"`
	AdminAccessKey string `json:"admin_access_key"`
	AdminSecretKey string `json:"admin_secret_key"`
}

func Default(dataDir string) Config {
	return Config{
		BindAddr:       ":9000",
		DataDir:        dataDir,
		AdminAccessKey: "sssadmin",
		AdminSecretKey: "sssadmin-secret-change-me",
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
