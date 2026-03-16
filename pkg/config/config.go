package config

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

type Config struct {
	ClashAPI    string
	ClashSecret string
}

// Load reads config from ~/.network_doctor/config.
// Returns empty config if file doesn't exist.
func Load() *Config {
	cfg := &Config{}

	home, err := os.UserHomeDir()
	if err != nil {
		return cfg
	}

	path := filepath.Join(home, ".network_doctor", "config")
	f, err := os.Open(path)
	if err != nil {
		return cfg
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		// Strip surrounding quotes
		val = strings.Trim(val, "\"'")

		switch key {
		case "clash-api":
			cfg.ClashAPI = val
		case "clash-secret":
			cfg.ClashSecret = val
		}
	}

	return cfg
}
