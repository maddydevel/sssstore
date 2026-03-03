package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadStrictModeValidation(t *testing.T) {
	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "cfg.json")
	cfg, err := Init(cfgPath, filepath.Join(tmp, "data"))
	if err != nil {
		t.Fatal(err)
	}
	cfg.StrictMode = true
	cfg.AdminSecretKey = "sssadmin-secret-change-me"
	b := []byte(`{"bind_addr":":9000","data_dir":"` + cfg.DataDir + `","admin_access_key":"` + cfg.AdminAccessKey + `","admin_secret_key":"` + cfg.AdminSecretKey + `","strict_mode":true}`)
	if err := os.WriteFile(cfgPath, b, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(cfgPath); err == nil {
		t.Fatal("expected strict mode validation error")
	}
}
