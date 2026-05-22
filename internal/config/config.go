package config

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

const (
	DirName    = ".regressguard"
	FileName   = "config.json"
	EnvFile    = ".env"
	IgnoreFile = "ignore"
)

type Config struct {
	Version        int      `json:"version"`
	ProjectRoot    string   `json:"projectRoot"`
	PackageManager string   `json:"packageManager"`
	Framework      string   `json:"framework"`
	TestCommand    string   `json:"testCommand"`
	ServerURL      string   `json:"serverUrl"`
	ServerCommand  string   `json:"serverCommand,omitempty"`
	Auth           Auth     `json:"auth"`
	IgnoreFields   []string `json:"ignoreFields"`
	Routes         []Route  `json:"routes"`
	MaxHistory     int      `json:"maxHistory,omitempty"`
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

// IgnorePath returns the path to .regressguard/ignore.
func IgnorePath(root string) string {
	return filepath.Join(root, DirName, IgnoreFile)
}

func Exists(root string) bool {
	_, err := os.Stat(Path(root))
	return err == nil
}

// Load reads and parses the config file from the project root.
// It auto-loads .regressguard/.env if present and resolves $VAR references
// in auth.testToken and auth.cookie fields.
// It also loads .regressguard/ignore if present and merges field/route rules.
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

	// F5: load .regressguard/ignore and merge rules.
	ignoreFields, ignoreRoutes := loadIgnoreFile(root)
	if len(ignoreFields) > 0 {
		cfg.IgnoreFields = mergeUnique(cfg.IgnoreFields, ignoreFields)
	}
	if len(ignoreRoutes) > 0 {
		cfg.Routes = applyRouteIgnores(cfg.Routes, ignoreRoutes)
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

// loadIgnoreFile reads .regressguard/ignore and returns field ignore rules
// and route skip patterns separately.
//
// Format:
//   - Lines starting with # are comments
//   - Blank lines are ignored
//   - "field:<name>" ignores a field during schema normalization
//   - "route:<METHOD> <path-glob>" skips matching routes
//   - Bare entries (no prefix) are treated as field ignores
//
// Examples:
//   # Ignore volatile fields
//   field:requestId
//   field:traceId
//   internalRef
//
//   # Skip admin routes
//   route:GET /api/admin/*
//   route:* /api/internal/*
func loadIgnoreFile(root string) (fields []string, routes []string) {
	f, err := os.Open(IgnorePath(root))
	if err != nil {
		return nil, nil
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		if strings.HasPrefix(line, "route:") {
			pattern := strings.TrimSpace(strings.TrimPrefix(line, "route:"))
			if pattern != "" {
				routes = append(routes, pattern)
			}
		} else if strings.HasPrefix(line, "field:") {
			field := strings.TrimSpace(strings.TrimPrefix(line, "field:"))
			if field != "" {
				fields = append(fields, field)
			}
		} else {
			// Bare entry — treat as field ignore.
			fields = append(fields, line)
		}
	}
	return fields, routes
}

// applyRouteIgnores marks routes as Skip=true if they match any ignore pattern.
// Patterns support glob matching: "GET /api/admin/*" or "* /api/internal/*".
func applyRouteIgnores(routes []Route, patterns []string) []Route {
	result := make([]Route, len(routes))
	copy(result, routes)

	for i, route := range result {
		if route.Skip {
			continue // already skipped
		}
		routeKey := route.Method + " " + route.Path
		for _, pattern := range patterns {
			if matchRoutePattern(routeKey, pattern) {
				result[i].Skip = true
				break
			}
		}
	}
	return result
}

// matchRoutePattern checks if a route key matches a pattern.
// Pattern format: "METHOD /path/glob" where METHOD can be "*" for any method.
// Path supports simple glob: * matches any segment.
func matchRoutePattern(routeKey, pattern string) bool {
	parts := strings.SplitN(pattern, " ", 2)
	if len(parts) != 2 {
		return false
	}
	method := strings.ToUpper(parts[0])
	pathPattern := parts[1]

	routeParts := strings.SplitN(routeKey, " ", 2)
	if len(routeParts) != 2 {
		return false
	}
	routeMethod := routeParts[0]
	routePath := routeParts[1]

	// Check method match.
	if method != "*" && method != routeMethod {
		return false
	}

	// Check path match using filepath.Match (supports * and ? globs).
	matched, _ := filepath.Match(pathPattern, routePath)
	return matched
}

// mergeUnique appends items from b to a, skipping duplicates.
func mergeUnique(a, b []string) []string {
	seen := make(map[string]bool, len(a))
	for _, s := range a {
		seen[s] = true
	}
	result := make([]string, len(a))
	copy(result, a)
	for _, s := range b {
		if !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}
	return result
}
