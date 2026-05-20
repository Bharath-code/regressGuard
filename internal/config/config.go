package config

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

const (
	DirName  = ".regressguard"
	FileName = "config.json"
	EnvFile  = ".env"
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
	Method string          `json:"method"`
	Path   string          `json:"path"`
	Skip   bool            `json:"skip,omitempty"`
	Body   json.RawMessage `json:"body,omitempty"`
}

func Path(root string) string {
	return filepath.Join(root, DirName, FileName)
}

// EnvPath returns the path to .regressguard/.env.
func EnvPath(root string) string {
	return filepath.Join(root, DirName, EnvFile)
}

func Exists(root string) bool {
	_, err := os.Stat(Path(root))
	return err == nil
}

// Load reads and parses the config file from the project root.
// It auto-loads .regressguard/.env if present and resolves $VAR references
// in auth.testToken and auth.cookie fields.
func Load(root string) (Config, error) {
	data, err := os.ReadFile(Path(root))
	if err != nil {
		return Config{}, err
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, err
	}

	// E13-T2: load .regressguard/.env and resolve $VAR references.
	loadEnvFile(root)
	cfg.Auth.TestToken = resolveEnv(cfg.Auth.TestToken)
	cfg.Auth.Cookie = resolveEnv(cfg.Auth.Cookie)

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

// resolveEnv resolves $VAR or ${VAR} references in a string from the environment.
// If the string doesn't start with $, it's returned as-is.
func resolveEnv(s string) string {
	if s == "" {
		return s
	}
	if !strings.HasPrefix(s, "$") {
		return s
	}
	// Strip $ and optional { }.
	varName := strings.TrimPrefix(s, "$")
	varName = strings.TrimPrefix(varName, "{")
	varName = strings.TrimSuffix(varName, "}")
	varName = strings.TrimSpace(varName)
	if varName == "" {
		return s
	}
	if val := os.Getenv(varName); val != "" {
		return val
	}
	// Return empty if env var not set (don't leak the $VAR literal).
	return ""
}

// loadEnvFile reads .regressguard/.env and sets environment variables.
// Format: KEY=VALUE (one per line, # comments, no export prefix).
func loadEnvFile(root string) {
	f, err := os.Open(EnvPath(root))
	if err != nil {
		return
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// Strip optional "export " prefix.
		line = strings.TrimPrefix(line, "export ")
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		// Strip surrounding quotes.
		value = strings.Trim(value, "\"'")
		// Only set if not already set in environment (env takes precedence).
		if os.Getenv(key) == "" {
			_ = os.Setenv(key, value)
		}
	}
}

// LooksLikeSecret returns true if the token value looks like a raw secret
// (not an env var reference). Used by rg doctor to warn users.
func LooksLikeSecret(token string) bool {
	if token == "" || strings.HasPrefix(token, "$") {
		return false
	}
	// JWT-like tokens, long hex strings, or anything over 20 chars.
	return len(token) > 20 || strings.Contains(token, "eyJ")
}
