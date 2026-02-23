// Package storage handles persistent storage for bot settings
package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// Settings represents the bot settings stored in the JSON file
type Settings struct {
	DailyWebhookEnabled bool `json:"daily_webhook_enabled"`
}

// Storage handles persistent storage of bot settings
type Storage struct {
	filename string
	settings Settings
	mutex    sync.RWMutex
}

// New creates a new Storage instance
func New(filename string) (*Storage, error) {
	s := &Storage{
		filename: filename,
		settings: Settings{
			DailyWebhookEnabled: false, // Default to disabled
		},
	}

	// Try to load existing settings
	if err := s.load(); err != nil {
		// If file doesn't exist, create it with default settings
		if os.IsNotExist(err) {
			if err := s.save(); err != nil {
				return nil, fmt.Errorf("failed to create settings file: %w", err)
			}
		} else {
			return nil, fmt.Errorf("failed to load settings: %w", err)
		}
	}

	return s, nil
}

// load reads settings from the JSON file
func (s *Storage) load() error {
	data, err := os.ReadFile(s.filename)
	if err != nil {
		return err
	}

	var settings Settings
	if err := json.Unmarshal(data, &settings); err != nil {
		return fmt.Errorf("failed to unmarshal settings: %w", err)
	}

	s.mutex.Lock()
	s.settings = settings
	s.mutex.Unlock()

	return nil
}

// save writes settings to the JSON file
func (s *Storage) save() error {
	s.mutex.RLock()
	data, err := json.MarshalIndent(s.settings, "", "  ")
	s.mutex.RUnlock()
	if err != nil {
		return fmt.Errorf("failed to marshal settings: %w", err)
	}

	// ✅ Create parent directory if it doesn't exist
	dir := filepath.Dir(s.filename)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	if err := os.WriteFile(s.filename, data, 0o644); err != nil {
		return fmt.Errorf("failed to write settings file: %w", err)
	}
	return nil
}

// GetDailyWebhookEnabled returns whether the daily webhook is enabled
func (s *Storage) GetDailyWebhookEnabled() bool {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.settings.DailyWebhookEnabled
}

// SetDailyWebhookEnabled sets whether the daily webhook is enabled
func (s *Storage) SetDailyWebhookEnabled(enabled bool) error {
	s.mutex.Lock()
	s.settings.DailyWebhookEnabled = enabled
	s.mutex.Unlock()

	return s.save()
}

// ToggleDailyWebhookEnabled toggles the daily webhook enabled status
func (s *Storage) ToggleDailyWebhookEnabled() (bool, error) {
	s.mutex.Lock()
	s.settings.DailyWebhookEnabled = !s.settings.DailyWebhookEnabled
	newState := s.settings.DailyWebhookEnabled
	s.mutex.Unlock()

	if err := s.save(); err != nil {
		return false, err
	}

	return newState, nil
}

// GetAllSettings returns all settings
func (s *Storage) GetAllSettings() Settings {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.settings
}

