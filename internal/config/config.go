package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"

	"github.com/google/uuid"
)

var ErrNotSet = errors.New("not set")

type file struct {
	APIKey    string `json:"api_key,omitempty"`
	InstallID string `json:"install_id,omitempty"`
}

var (
	configOnce sync.Once
	cachedFile file
	cachedErr  error
)

func configPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".gcm", "config.json"), nil
}

func loadFromDisk() (file, error) {
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

func load() (file, error) {
	configOnce.Do(func() {
		cachedFile, cachedErr = loadFromDisk()
	})
	return cachedFile, cachedErr
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
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// isDisabledEnv reports whether telemetry should be suppressed based on
// environment variables. Intentionally duplicated from internal/telemetry
// to avoid a circular import.
func isDisabledEnv() bool {
	return os.Getenv("CI") != "" ||
		os.Getenv("GITHUB_ACTIONS") != ""
}

// GetOrCreateInstallID returns the stored anonymous install ID, generating
// one if absent. Returns ("", nil) in CI/automation environments — callers
// pass the empty string to telemetry.New(), which returns a noop recorder.
func GetOrCreateInstallID() (string, error) {
	if isDisabledEnv() {
		return "", nil
	}
	f, err := load()
	if err != nil {
		return "", err
	}
	if f.InstallID != "" {
		return f.InstallID, nil
	}
	f.InstallID = uuid.New().String()
	cachedFile = f // keep in-process cache consistent
	return f.InstallID, save(f)
}

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

func SetAPIKey(key string) error {
	f, err := load()
	if err != nil {
		return err
	}
	f.APIKey = key
	cachedFile = f // write-through so in-process reads see the new key immediately
	return save(f)
}

func UnsetAPIKey() error {
	f, err := load()
	if err != nil {
		return err
	}
	f.APIKey = ""
	cachedFile = f // write-through so in-process reads see the cleared key immediately
	return save(f)
}
