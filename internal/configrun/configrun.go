// Package configrun implements rg config get and rg config set.
// It supports dotted paths for nested fields (e.g. auth.testToken).
package configrun

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/Bharath-code/regressguard/internal/config"
	"github.com/Bharath-code/regressguard/internal/failures"
)

// Get reads a config value by key and writes it to stdout.
// Supports dotted paths: serverUrl, testCommand, auth.testToken, auth.mode, etc.
func Get(key string, root string, stdout io.Writer) error {
	cfg, err := loadConfig(root)
	if err != nil {
		return err
	}

	val, ok := getField(cfg, key)
	if !ok {
		return failures.Actionable{
			Title:       fmt.Sprintf("rg config get failed: unknown key %q.", key),
			Cause:       "This key does not exist in the config schema.",
			Next:        "rg config --help",
			MoreContext: "rg config get --help",
		}
	}

	_, err = fmt.Fprintln(stdout, val)
	return err
}

// Set writes a config value by key and saves the config file.
func Set(key, value, root string, stdout io.Writer) error {
	cfg, err := loadConfig(root)
	if err != nil {
		return err
	}

	if !setField(&cfg, key, value) {
		return failures.Actionable{
			Title:       fmt.Sprintf("rg config set failed: unknown key %q.", key),
			Cause:       "This key does not exist in the config schema.",
			Next:        "rg config --help",
			MoreContext: "rg config set --help",
		}
	}

	if err := config.Write(root, cfg); err != nil {
		return fmt.Errorf("write config: %w", err)
	}

	_, _ = fmt.Fprintf(stdout, "OK Set %s = %s\n", key, value)
	_, _ = fmt.Fprintf(stdout, "   %s\n", config.Path(root))
	return nil
}

func loadConfig(root string) (config.Config, error) {
	if !config.Exists(root) {
		return config.Config{}, failures.Actionable{
			Title:       "rg config failed: no config found.",
			Cause:       "RegressGuard has not been initialized for this project.",
			Next:        "rg init",
			MoreContext: "rg config --help",
		}
	}
	return config.Load(root)
}

// getField reads a value from the config by dotted key path.
func getField(cfg config.Config, key string) (string, bool) {
	switch strings.ToLower(key) {
	case "serverurl", "server-url":
		return cfg.ServerURL, true
	case "testcommand", "test-command":
		return cfg.TestCommand, true
	case "framework":
		return cfg.Framework, true
	case "packagemanager", "package-manager":
		return cfg.PackageManager, true
	case "auth.mode":
		return cfg.Auth.Mode, true
	case "auth.testtoken":
		return cfg.Auth.TestToken, true
	case "auth.headername":
		return cfg.Auth.HeaderName, true
	case "auth.prefix":
		return cfg.Auth.Prefix, true
	case "auth.cookie":
		return cfg.Auth.Cookie, true
	case "ignorefields":
		if len(cfg.IgnoreFields) == 0 {
			return "[]", true
		}
		b, _ := json.Marshal(cfg.IgnoreFields)
		return string(b), true
	default:
		return "", false
	}
}

// setField writes a value to the config by dotted key path.
func setField(cfg *config.Config, key, value string) bool {
	switch strings.ToLower(key) {
	case "serverurl", "server-url":
		cfg.ServerURL = value
	case "testcommand", "test-command":
		cfg.TestCommand = value
	case "framework":
		cfg.Framework = value
	case "packagemanager", "package-manager":
		cfg.PackageManager = value
	case "auth.mode":
		cfg.Auth.Mode = value
	case "auth.testtoken":
		cfg.Auth.TestToken = value
	case "auth.headername":
		cfg.Auth.HeaderName = value
	case "auth.prefix":
		cfg.Auth.Prefix = value
	case "auth.cookie":
		cfg.Auth.Cookie = value
	case "ignorefields":
		// Accept JSON array or comma-separated list.
		value = strings.TrimSpace(value)
		if strings.HasPrefix(value, "[") {
			var fields []string
			if err := json.Unmarshal([]byte(value), &fields); err == nil {
				cfg.IgnoreFields = fields
			} else {
				cfg.IgnoreFields = splitCSV(value)
			}
		} else {
			cfg.IgnoreFields = splitCSV(value)
		}
	default:
		return false
	}
	return true
}

func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	var result []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

// WithDefaults fills in the root if empty.
func WithDefaults(root string) string {
	if root == "" {
		return "."
	}
	return root
}

// Ensure stdout defaults.
func DefaultStdout(w io.Writer) io.Writer {
	if w == nil {
		return os.Stdout
	}
	return w
}
