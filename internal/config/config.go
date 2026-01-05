package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/ini.v1"
)

// ClientConfig holds the local settings.
type ClientConfig struct {
	General GeneralConfig `ini:"General"`
	Theme   ThemeConfig   `ini:"Theme"`
	Network NetworkConfig `ini:"Network"`
	Session SessionConfig `ini:"Session"`
}

type GeneralConfig struct {
	Debug bool `ini:"Debug"`
}

type ThemeConfig struct {
	Color string `ini:"Color"` // Default: "Green"
}

type NetworkConfig struct {
	Timeout int `ini:"Timeout"` // Seconds
}

type SessionConfig struct {
	LastHost string `ini:"LastHost"`
	LastUser string `ini:"LastUser"`
	LastPort int    `ini:"LastPort"`
}

// DefaultClientConfig returns standard defaults.
func DefaultClientConfig() *ClientConfig {
	return &ClientConfig{
		General: GeneralConfig{
			Debug: false,
		},
		Theme: ThemeConfig{
			Color: "Green",
		},
		Network: NetworkConfig{
			Timeout: 30,
		},
		Session: SessionConfig{
			LastPort: 22,
		},
	}
}

// LoadClientConfig reads client.ini from the executable directory.
func LoadClientConfig() (*ClientConfig, error) {
	exePath, err := os.Executable()
	if err != nil {
		return nil, err
	}
	configPath := filepath.Join(filepath.Dir(exePath), "client.ini")

	// If not exists, create default
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		cfg := DefaultClientConfig()
		if err := SaveClientConfig(cfg); err != nil {
			return nil, err
		}
		return cfg, nil
	}

	cfg := new(ClientConfig)
	err = ini.MapTo(cfg, configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to parse client.ini: %w", err)
	}
	return cfg, nil
}

// SaveClientConfig writes the config to disk.
func SaveClientConfig(cfg *ClientConfig) error {
	exePath, err := os.Executable()
	if err != nil {
		return err
	}
	configPath := filepath.Join(filepath.Dir(exePath), "client.ini")

	iniFile := ini.Empty()
	err = ini.ReflectFrom(iniFile, cfg)
	if err != nil {
		return err
	}
	return iniFile.SaveTo(configPath)
}
