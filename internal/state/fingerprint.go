package state

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

const (
	maxFingerprintFiles = 50000
	maxFingerprintBytes = 20 << 20
)

// TreeFingerprint hashes the project's working tree: every tracked and
// untracked-but-not-ignored file, by path and content, except RegressGuard's
// own files (.regressguard/, the root .gitignore that init edits). Two equal
// fingerprints mean identical code. It returns "" when it cannot be sure
// (not a git repo, git failure, unreadable or oversized tree), so callers
// must treat "" as "unknown" and fail closed.
func TreeFingerprint(root string) string {
	out, err := exec.Command("git", "-C", root, "ls-files", "-z", "--cached", "--others", "--exclude-standard").Output()
	if err != nil {
		return ""
	}
	paths := strings.Split(strings.TrimRight(string(out), "\x00"), "\x00")
	if len(paths) > maxFingerprintFiles {
		return ""
	}
	h := sha256.New()
	for _, p := range paths {
		if p == "" || p == ".gitignore" || strings.HasPrefix(p, ".regressguard/") {
			continue
		}
		h.Write([]byte(p))
		h.Write([]byte{0})
		data, err := os.ReadFile(root + string(os.PathSeparator) + p)
		switch {
		case os.IsNotExist(err):
			h.Write([]byte("<deleted>"))
		case err != nil || len(data) > maxFingerprintBytes:
			return ""
		default:
			h.Write([]byte(strconv.Itoa(len(data))))
			h.Write([]byte{0})
			h.Write(data)
		}
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}
