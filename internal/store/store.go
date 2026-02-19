package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const (
	StoreVersion          = "1.0"
	UncategorizedName     = "Uncategorized"
	StoreFileName         = "gcm.json"
	CategoryNameMaxLength = 64
)

var (
	ErrNotInitialized     = errors.New("gcm not initialized: run 'gcm init' first")
	ErrAlreadyInitialized = errors.New("gcm already initialized in this repository")
	ErrInvalidName        = errors.New("invalid category name")
	ErrCategoryExists     = errors.New("category already exists")
)

type Category struct {
	Name      string `json:"name"`
	Immutable bool   `json:"immutable"`
}

type Store struct {
	Version     string            `json:"version"`
	Categories  []Category        `json:"categories"`
	Assignments map[string]string `json:"assignments"`
}

func NewStore() *Store {
	return &Store{
		Version: StoreVersion,
		Categories: []Category{
			{
				Name:      UncategorizedName,
				Immutable: true,
			},
		},
		Assignments: make(map[string]string),
	}
}

func StorePath(gitDir string) string {
	return filepath.Join(gitDir, StoreFileName)
}

func Save(gitDir string, s *Store) error {
	path := StorePath(gitDir)
	tmpPath := path + ".tmp"

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}

	if err := os.WriteFile(tmpPath, data, 0o644); err != nil {
		return err
	}

	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return err
	}

	return nil
}

func Load(gitDir string) (*Store, error) {
	path := StorePath(gitDir)
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, ErrNotInitialized
		}
		return nil, err
	}

	var s Store
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", path, err)
	}
	return &s, nil
}

func ValidateCategoryName(name string) error {
	if strings.TrimSpace(name) == "" {
		return ErrInvalidName
	}

	if len(name) > CategoryNameMaxLength {
		return ErrInvalidName
	}

	if name == UncategorizedName {
		return ErrInvalidName
	}

	if !regexp.MustCompile(`^[a-zA-Z0-9-]+$`).MatchString(name) {
		return ErrInvalidName
	}

	return nil
}

func (s *Store) CategoryExists(name string) bool {
	for _, cat := range s.Categories {
		if cat.Name == name {
			return true
		}
	}
	return false
}

func (s *Store) AddCategory(name string) error {
	if err := ValidateCategoryName(name); err != nil {
		return err
	}

	if s.CategoryExists(name) {
		return ErrCategoryExists
	}

	s.Categories = append(s.Categories, Category{Name: name, Immutable: false})
	return nil
}
