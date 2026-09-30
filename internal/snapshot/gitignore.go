package snapshot

import (
	"os"
	"path/filepath"
	"strings"
)

const (
	ignoreAll  = DirName + "/*"
	ignoreKeep = "!" + DirName + "/" + FileName
)

// LegacyIgnore reports whether .gitignore still ignores the whole directory,
// which hides snapshot.json from git.
func LegacyIgnore(root string) bool {
	data, err := os.ReadFile(filepath.Join(root, ".gitignore"))
	if err != nil {
		return false
	}
	for _, l := range strings.Split(string(data), "\n") {
		if t := strings.TrimSpace(l); t == DirName+"/" || t == DirName || t == "/"+DirName+"/" {
			return true
		}
	}
	return false
}

// EnsureGitignore makes .gitignore ignore .regressguard/* but keep snapshot.json
// trackable. Idempotent; replaces a legacy whole-directory ignore in place.
func EnsureGitignore(root string) error {
	p := filepath.Join(root, ".gitignore")
	data, err := os.ReadFile(p)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	content := string(data)
	if strings.Contains(content, ignoreKeep) {
		return nil
	}
	rule := ignoreAll + "\n" + ignoreKeep
	var out []string
	replaced := false
	for _, l := range strings.Split(strings.TrimRight(content, "\n"), "\n") {
		if t := strings.TrimSpace(l); t == DirName+"/" || t == DirName || t == "/"+DirName+"/" {
			if !replaced {
				out = append(out, rule)
				replaced = true
			}
			continue
		}
		out = append(out, l)
	}
	if !replaced {
		if content == "" {
			out = nil
		}
		out = append(out, rule)
	}
	return os.WriteFile(p, []byte(strings.Join(out, "\n")+"\n"), 0o644)
}
