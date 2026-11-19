package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Config holds user-persisted settings.
type Config struct {
	JavaBin   string `json:"java_bin"`
	ProxyHost string `json:"proxy_host"`
	ProxyPort string `json:"proxy_port"`
	// AutoAttach re-attaches watched JVMs when they restart with a new PID.
	AutoAttach bool `json:"auto_attach"`
	// AutoAttachPatterns are process names (main class or jar) to watch.
	AutoAttachPatterns []string `json:"auto_attach_patterns"`
}

// Load reads settings from dir/settings.json. Missing fields keep their zero
// values so callers can apply defaults afterwards.
func Load(dir string) (*Config, error) {
	data, err := os.ReadFile(filepath.Join(dir, "settings.json"))
	if err != nil {
		if os.IsNotExist(err) {
			return &Config{}, nil
		}
		return nil, err
	}
	var c Config
	if err := json.Unmarshal(data, &c); err != nil {
		return &Config{}, nil // corrupt file → start fresh
	}
	return &c, nil
}

// Save writes the config to dir/settings.json.
func (c *Config) Save(dir string) error {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "settings.json"), data, 0600)
}
