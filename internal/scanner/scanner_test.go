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

// --- E10-T5: Express route discovery ---

func TestDetectFramework_Express(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "package.json"), `{"dependencies":{"express":"^4.18.0"},"scripts":{"test":"jest"}}`)

	pkg, _ := readPackageJSON(root)
	framework := DetectFramework(root, pkg)
	if framework != "express" {
		t.Fatalf("framework = %q, want express", framework)
	}
}

func TestDetectFramework_Hono(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "package.json"), `{"dependencies":{"hono":"^4.0.0"},"scripts":{"test":"vitest"}}`)

	pkg, _ := readPackageJSON(root)
	framework := DetectFramework(root, pkg)
	if framework != "hono" {
		t.Fatalf("framework = %q, want hono", framework)
	}
}

func TestDiscoverExpressRoutes_basic(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "package.json"), `{"dependencies":{"express":"^4.18.0"},"scripts":{"test":"jest"}}`)
	writeFile(t, filepath.Join(root, "src", "index.ts"), `
import express from "express";
const app = express();

app.get("/api/health", (req, res) => {
  res.json({ status: "ok" });
});

app.get("/api/users", (req, res) => {
  res.json({ users: [] });
});

app.post("/api/users", (req, res) => {
  res.status(201).json({ id: 1 });
});

app.delete("/api/users/:id", (req, res) => {
  res.status(204).end();
});
`)

	routes, err := DiscoverExpressRoutes(root)
	if err != nil {
		t.Fatal(err)
	}

	if len(routes) != 4 {
		t.Fatalf("expected 4 routes, got %d: %#v", len(routes), routes)
	}

	// Routes should be sorted by path then method.
	expected := []Route{
		{Method: "GET", Path: "/api/health"},
		{Method: "GET", Path: "/api/users"},
		{Method: "POST", Path: "/api/users"},
		{Method: "DELETE", Path: "/api/users/:id"},
	}
	for i, want := range expected {
		if routes[i].Method != want.Method || routes[i].Path != want.Path {
			t.Errorf("route[%d] = %s %s, want %s %s", i, routes[i].Method, routes[i].Path, want.Method, want.Path)
		}
	}
}

func TestDiscoverExpressRoutes_router(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "package.json"), `{"dependencies":{"express":"^4.18.0"}}`)
	writeFile(t, filepath.Join(root, "routes", "api.ts"), `
import { Router } from "express";
const router = Router();

router.get("/api/health", handler);
router.post("/api/auth/login", loginHandler);
router.put("/api/profile", updateProfile);
`)

	routes, err := DiscoverExpressRoutes(root)
	if err != nil {
		t.Fatal(err)
	}

	if len(routes) != 3 {
		t.Fatalf("expected 3 routes, got %d: %#v", len(routes), routes)
	}

	// Verify specific routes found.
	found := map[string]bool{}
	for _, r := range routes {
		found[r.Method+" "+r.Path] = true
	}
	for _, want := range []string{"GET /api/health", "POST /api/auth/login", "PUT /api/profile"} {
		if !found[want] {
			t.Errorf("missing route: %s", want)
		}
	}
}

func TestDiscoverExpressRoutes_hono(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "package.json"), `{"dependencies":{"hono":"^4.0.0"}}`)
	writeFile(t, filepath.Join(root, "src", "index.ts"), `
import { Hono } from "hono";
const app = new Hono();

app.get("/api/users", (c) => c.json({ users: [] }));
app.post("/api/users", (c) => c.json({ id: 1 }));
app.get("/api/health", (c) => c.json({ status: "ok" }));
`)

	routes, err := DiscoverExpressRoutes(root)
	if err != nil {
		t.Fatal(err)
	}

	if len(routes) != 3 {
		t.Fatalf("expected 3 routes, got %d: %#v", len(routes), routes)
	}
}

func TestDiscoverExpressRoutes_skipsNodeModules(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "package.json"), `{"dependencies":{"express":"^4.18.0"}}`)
	writeFile(t, filepath.Join(root, "src", "app.ts"), `app.get("/api/real", handler);`)
	writeFile(t, filepath.Join(root, "node_modules", "some-pkg", "index.js"), `app.get("/api/fake", handler);`)

	routes, err := DiscoverExpressRoutes(root)
	if err != nil {
		t.Fatal(err)
	}

	for _, r := range routes {
		if r.Path == "/api/fake" {
			t.Error("should not discover routes from node_modules")
		}
	}
	if len(routes) != 1 || routes[0].Path != "/api/real" {
		t.Fatalf("expected only /api/real, got: %#v", routes)
	}
}

func TestDiscoverExpressRoutes_deduplicates(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "package.json"), `{"dependencies":{"express":"^4.18.0"}}`)
	// Same route defined in two files.
	writeFile(t, filepath.Join(root, "src", "routes.ts"), `app.get("/api/health", handler);`)
	writeFile(t, filepath.Join(root, "src", "app.ts"), `app.get("/api/health", otherHandler);`)

	routes, err := DiscoverExpressRoutes(root)
	if err != nil {
		t.Fatal(err)
	}

	if len(routes) != 1 {
		t.Fatalf("expected 1 deduplicated route, got %d: %#v", len(routes), routes)
	}
}

func TestDetect_expressIntegration(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "package.json"), `{
		"dependencies": {"express": "^4.18.0"},
		"scripts": {"test": "jest"}
	}`)
	writeFile(t, filepath.Join(root, "src", "index.ts"), `
import express from "express";
const app = express();
app.get("/api/users", handler);
app.get("/api/health", handler);
`)

	detection, err := Detect(root, "")
	if err != nil {
		t.Fatal(err)
	}

	if detection.Framework != "express" {
		t.Fatalf("framework = %q, want express", detection.Framework)
	}
	if len(detection.Routes) != 2 {
		t.Fatalf("expected 2 routes, got %d: %#v", len(detection.Routes), detection.Routes)
	}

	// Verify /api/health and /api/users are discovered.
	found := map[string]bool{}
	for _, r := range detection.Routes {
		found[r.Path] = true
	}
	if !found["/api/users"] || !found["/api/health"] {
		t.Fatalf("expected /api/users and /api/health, got: %#v", detection.Routes)
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
