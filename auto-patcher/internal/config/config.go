package config

import (
	"bytes"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/getsops/sops/v3/decrypt"
	"gopkg.in/yaml.v3"
)

type Config struct {
	Org          string        `yaml:"org"`
	Workers      int           `yaml:"workers"`
	ScanInterval time.Duration `yaml:"scan_interval"`
	WorkerDelay  time.Duration `yaml:"worker_delay"`
	Exclude      []string      `yaml:"exclude"`

	GitHub struct {
		Token string `yaml:"token"`
	} `yaml:"github"`

	Anthropic struct {
		Token string `yaml:"token"`
	} `yaml:"anthropic"`
}

func Load(path string) (*Config, error) {
	data, err := readConfig(path)
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}

	cfg := &Config{
		Workers:      3,
		ScanInterval: 15 * time.Minute,
		WorkerDelay:  10 * time.Minute,
	}
	if err := yaml.NewDecoder(bytes.NewReader(data)).Decode(cfg); err != nil {
		return nil, fmt.Errorf("decode config: %w", err)
	}
	return cfg, nil
}

// readConfig attempts to decrypt path with the sops library. If the file has
// no sops metadata, it falls back to reading plaintext.
func readConfig(path string) ([]byte, error) {
	out, err := decrypt.File(path, "yaml")
	if err == nil {
		slog.Debug("loaded config via sops", "path", path)
		return out, nil
	}
	slog.Debug("file not sops-encrypted, reading plaintext", "path", path)
	return os.ReadFile(path)
}

func (c *Config) GitHubToken() string  { return c.GitHub.Token }
func (c *Config) AnthropicKey() string { return c.Anthropic.Token }
