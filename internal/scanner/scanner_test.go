package scanner

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindRootFindsNearestPackageJSON(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "package.json"), `{"scripts":{"test":"vitest"}}`)
	nested := filepath.Join(root, "src", "lib")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}

	got, err := FindRoot(nested)
	if err != nil {
		t.Fatal(err)
	}
	if got != root {
		t.Fatalf("root = %q, want %q", got, root)
	}
}

func TestFindRootFallsBackToGitRoot(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}

	got, err := FindRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	if got != root {
		t.Fatalf("root = %q, want %q", got, root)
	}
}

func TestFindRootFailsOutsideProject(t *testing.T) {
	_, err := FindRoot(t.TempDir())
	if err == nil {
		t.Fatal("expected root detection failure")
	}
}

func TestDetectPackageManagerMatrix(t *testing.T) {
	tests := map[string]string{
		"bun.lock":          "bun",
		"pnpm-lock.yaml":    "pnpm",
		"yarn.lock":         "yarn",
		"package-lock.json": "npm",
	}
	for lockfile, want := range tests {
		t.Run(lockfile, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "package.json"), `{}`)
			writeFile(t, filepath.Join(root, lockfile), "")
			if got := DetectPackageManager(root); got != want {
				t.Fatalf("package manager = %q, want %q", got, want)
			}
		})
	}
}

func TestDetectInfersTestCommand(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "package.json"), `{
		"scripts": {"test": "vitest run"},
		"devDependencies": {"vitest": "^1.0.0"}
	}`)

	detection, err := Detect(root, "")
	if err != nil {
		t.Fatal(err)
	}
	if detection.TestCommand != "npm test" {
		t.Fatalf("test command = %q, want npm test", detection.TestCommand)
	}
}

func TestDetectHonorsTestCommandOverride(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "package.json"), `{}`)

	detection, err := Detect(root, "pnpm test -- --runInBand")
	if err != nil {
		t.Fatal(err)
	}
	if detection.TestCommand != "pnpm test -- --runInBand" {
		t.Fatalf("test command = %q", detection.TestCommand)
	}
}

func TestDetectsNextAppRouterAndRoutes(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "package.json"), `{"dependencies":{"next":"^15.0.0"},"scripts":{"test":"jest"}}`)
	writeFile(t, filepath.Join(root, "app", "api", "users", "[id]", "route.ts"), `
		export async function GET() {}
		export async function POST() {}
	`)

	detection, err := Detect(root, "")
	if err != nil {
		t.Fatal(err)
	}
	if detection.Framework != "nextjs-app-router" {
		t.Fatalf("framework = %q", detection.Framework)
	}
	if len(detection.Routes) != 2 {
		t.Fatalf("routes = %#v", detection.Routes)
	}
	if detection.Routes[0].Path != "/api/users/:id" {
		t.Fatalf("route path = %q", detection.Routes[0].Path)
	}
}

func writeFile(t *testing.T, path string, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}
