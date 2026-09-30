package upgraderun

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func releaseServer(t *testing.T, archive []byte, checksums *string) {
	t.Helper()
	name := fmt.Sprintf("rg_9.9.9_%s_%s.tar.gz", runtime.GOOS, runtime.GOARCH)
	mux := http.NewServeMux()
	var srv *httptest.Server
	mux.HandleFunc("/release", func(w http.ResponseWriter, r *http.Request) {
		assets := fmt.Sprintf(`{"name":%q,"browser_download_url":"%s/a"}`, name, srv.URL)
		if checksums != nil {
			assets += fmt.Sprintf(`,{"name":"checksums.txt","browser_download_url":"%s/c"}`, srv.URL)
		}
		fmt.Fprintf(w, `{"tag_name":"v9.9.9","assets":[%s]}`, assets)
	})
	mux.HandleFunc("/a", func(w http.ResponseWriter, r *http.Request) { w.Write(archive) })
	mux.HandleFunc("/c", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, strings.ReplaceAll(*checksums, "NAME", name))
	})
	srv = httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	old := latestURL
	latestURL = srv.URL + "/release"
	t.Cleanup(func() { latestURL = old })
}

func TestRun_missingChecksumsFails(t *testing.T) {
	releaseServer(t, []byte("x"), nil)
	_, err := Run(Options{CurrentVersion: "0.0.1", Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}})
	if err == nil || !strings.Contains(err.Error(), "checksums.txt") {
		t.Fatalf("want missing-checksums error, got %v", err)
	}
}

func TestRun_checksumMismatchFails(t *testing.T) {
	bad := strings.Repeat("0", 64) + "  NAME\n"
	releaseServer(t, []byte("x"), &bad)
	_, err := Run(Options{CurrentVersion: "0.0.1", Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}})
	if err == nil || !strings.Contains(err.Error(), "checksum verification failed") {
		t.Fatalf("want checksum error, got %v", err)
	}
}

func TestVerifyChecksum_valid(t *testing.T) {
	data := []byte("archive-bytes")
	sum := sha256.Sum256(data)
	body := hex.EncodeToString(sum[:]) + "  a.tar.gz\n"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, body) }))
	defer srv.Close()
	p := filepath.Join(t.TempDir(), "a.tar.gz")
	os.WriteFile(p, data, 0o644)
	if err := verifyChecksum(srv.URL, p, "a.tar.gz"); err != nil {
		t.Fatal(err)
	}
}

func TestReplaceBinary(t *testing.T) {
	d := t.TempDir()
	src, dst := filepath.Join(d, "new"), filepath.Join(d, "old")
	os.WriteFile(src, []byte("new"), 0o755)
	os.WriteFile(dst, []byte("old"), 0o755)
	if err := replaceBinary(src, dst); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(dst); string(b) != "new" {
		t.Fatalf("got %q", b)
	}
}
