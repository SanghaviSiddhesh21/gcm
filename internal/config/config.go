package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

// ErrNotSet is returned when the requested key has no value.
var ErrNotSet = errors.New("not set")

type file struct {
	APIKey string `json:"api_key,omitempty"`
}

func configPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".gcm", "config.json"), nil
}

func load() (file, error) {
	path, err := configPath()
	if err != nil {
		return file{}, err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return file{}, nil
	}
	if err != nil {
		return file{}, err
	}
	var f file
	return f, json.Unmarshal(data, &f)
}

func save(f file) error {
	path, err := configPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

// GetAPIKey returns the stored API key, or ErrNotSet if absent.
func GetAPIKey() (string, error) {
	f, err := load()
	if err != nil {
		return "", err
	}
	if f.APIKey == "" {
		return "", ErrNotSet
	}
	return f.APIKey, nil
}

// SetAPIKey stores the API key.
func SetAPIKey(key string) error {
	f, err := load()
	if err != nil {
		return err
	}
	f.APIKey = key
	return save(f)
}

// UnsetAPIKey removes the API key.
func UnsetAPIKey() error {
	f, err := load()
	if err != nil {
		return err
	}
	f.APIKey = ""
	return save(f)
}
