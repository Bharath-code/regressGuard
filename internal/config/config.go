package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

const (
	DirName  = ".regressguard"
	FileName = "config.json"
)

type Config struct {
	Version        int      `json:"version"`
	ProjectRoot    string   `json:"projectRoot"`
	PackageManager string   `json:"packageManager"`
	Framework      string   `json:"framework"`
	TestCommand    string   `json:"testCommand"`
	ServerURL      string   `json:"serverUrl"`
	Auth           Auth     `json:"auth"`
	IgnoreFields   []string `json:"ignoreFields"`
	Routes         []Route  `json:"routes"`
}

type Auth struct {
	Mode       string `json:"mode"`
	TestToken  string `json:"testToken,omitempty"`
	HeaderName string `json:"headerName,omitempty"`
	Prefix     string `json:"prefix,omitempty"`
	Cookie     string `json:"cookie,omitempty"`
}

type Route struct {
	Method string `json:"method"`
	Path   string `json:"path"`
	Skip   bool   `json:"skip,omitempty"`
}

func Path(root string) string {
	return filepath.Join(root, DirName, FileName)
}

func Exists(root string) bool {
	_, err := os.Stat(Path(root))
	return err == nil
}

// Load reads and parses the config file from the project root.
func Load(root string) (Config, error) {
	data, err := os.ReadFile(Path(root))
	if err != nil {
		return Config{}, err
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func Write(root string, cfg Config) error {
	dir := filepath.Join(root, DirName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(Path(root), data, 0o644)
}
