// S7: Snapshot integrity check using HMAC.
// Detects manual tampering of snapshot.json outside of rg snapshot.
package snapshot

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
)

const (
	HMACFileName = "snapshot.hmac"
	// hmacSecret is derived from the project path to make it project-specific.
	// This isn't meant to be cryptographically secret — it's a tamper-detection
	// mechanism, not an encryption scheme. The goal is to detect accidental or
	// casual edits to snapshot.json.
	hmacSalt = "regressguard-integrity-v1"
)

// HMACPath returns the path to the HMAC file.
func HMACPath(root string) string {
	return filepath.Join(root, DirName, HMACFileName)
}

// WriteHMAC computes and stores an HMAC for the current snapshot.json.
// Called automatically after snapshot.Write succeeds.
func WriteHMAC(root string) error {
	snapshotData, err := os.ReadFile(Path(root))
	if err != nil {
		return fmt.Errorf("read snapshot for HMAC: %w", err)
	}

	mac := computeHMAC(snapshotData, deriveKey(root))
	return os.WriteFile(HMACPath(root), []byte(mac+"\n"), 0o644)
}

// VerifyHMAC checks if the stored HMAC matches the current snapshot.json content.
// Returns:
//   - true, nil: HMAC matches (snapshot is untampered)
//   - false, nil: HMAC mismatch (snapshot was modified outside rg snapshot)
//   - true, error: HMAC file doesn't exist (no integrity data yet — not an error)
func VerifyHMAC(root string) (bool, error) {
	// Read the stored HMAC.
	storedMAC, err := os.ReadFile(HMACPath(root))
	if err != nil {
		if os.IsNotExist(err) {
			// No HMAC file yet — snapshot was created before S7 was implemented.
			return true, fmt.Errorf("no HMAC file (snapshot predates integrity check)")
		}
		return true, fmt.Errorf("read HMAC file: %w", err)
	}

	// Read the current snapshot.
	snapshotData, err := os.ReadFile(Path(root))
	if err != nil {
		return true, fmt.Errorf("read snapshot: %w", err)
	}

	// Compute expected HMAC.
	expected := computeHMAC(snapshotData, deriveKey(root))

	// Compare (trim whitespace from stored value).
	stored := string(storedMAC)
	if len(stored) > 0 && stored[len(stored)-1] == '\n' {
		stored = stored[:len(stored)-1]
	}

	return hmac.Equal([]byte(expected), []byte(stored)), nil
}

// computeHMAC generates an HMAC-SHA256 hex string.
func computeHMAC(data []byte, key []byte) string {
	mac := hmac.New(sha256.New, key)
	mac.Write(data)
	return hex.EncodeToString(mac.Sum(nil))
}

// deriveKey creates a project-specific HMAC key from the project root path.
// This ensures HMACs from different projects aren't interchangeable.
func deriveKey(root string) []byte {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		absRoot = root
	}
	h := sha256.New()
	h.Write([]byte(hmacSalt))
	h.Write([]byte(absRoot))
	return h.Sum(nil)
}
