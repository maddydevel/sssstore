package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type User struct {
	Name      string `json:"name"`
	AccessKey string `json:"access_key"`
	SecretKey string `json:"secret_key"`
}

func UsersFile(dataDir string) string { return filepath.Join(dataDir, "users.json") }

func LoadUsers(dataDir string) ([]User, error) {
	p := UsersFile(dataDir)
	b, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var users []User
	if err := json.Unmarshal(b, &users); err != nil {
		return nil, err
	}
	return users, nil
}

func SaveUsers(dataDir string, users []User) error {
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(users, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(UsersFile(dataDir), b, 0o600)
}
