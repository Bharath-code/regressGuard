package scanner

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

type Detection struct {
	Root           string
	PackageManager string
	Framework      string
	TestCommand    string
	Routes         []Route
}

type Route struct {
	Method string
	Path   string
}

type packageJSON struct {
	Scripts      map[string]string `json:"scripts"`
	Dependencies map[string]string `json:"dependencies"`
	DevDeps      map[string]string `json:"devDependencies"`
}

func Detect(start string, testOverride string) (Detection, error) {
	root, err := FindRoot(start)
	if err != nil {
		return Detection{}, err
	}
	pkg, _ := readPackageJSON(root)
	pm := DetectPackageManager(root)
	framework := DetectFramework(root, pkg)

	var routes []Route
	switch framework {
	case "nextjs-app-router":
		routes, _ = DiscoverNextAppRoutes(root)
	case "express", "hono":
		routes, _ = DiscoverExpressRoutes(root)
	}

	// W5: supplement with routes discovered from test files.
	testRoutes := DiscoverRoutesFromTests(root)
	if len(testRoutes) > 0 {
		seen := map[string]bool{}
		for _, r := range routes {
			seen[r.Method+" "+r.Path] = true
		}
		for _, r := range testRoutes {
			key := r.Method + " " + r.Path
			if !seen[key] {
				seen[key] = true
				routes = append(routes, r)
			}
		}
	}

	testCommand := strings.TrimSpace(testOverride)
	if testCommand == "" {
		testCommand = DetectTestCommand(pm, pkg)
	}
	return Detection{
		Root:           root,
		PackageManager: pm,
		Framework:      framework,
		TestCommand:    testCommand,
		Routes:         routes,
	}, nil
}

func FindRoot(start string) (string, error) {
	current, err := filepath.Abs(start)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(current)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		current = filepath.Dir(current)
	}
	for {
		if exists(filepath.Join(current, "package.json")) || exists(filepath.Join(current, ".git")) {
			return current, nil
		}
		parent := filepath.Dir(current)
		if parent == current {
			return "", errors.New("project root not found")
		}
		current = parent
	}
}

func DetectPackageManager(root string) string {
	switch {
	case exists(filepath.Join(root, "bun.lock")) || exists(filepath.Join(root, "bun.lockb")):
		return "bun"
	case exists(filepath.Join(root, "pnpm-lock.yaml")):
		return "pnpm"
	case exists(filepath.Join(root, "yarn.lock")):
		return "yarn"
	case exists(filepath.Join(root, "package-lock.json")):
		return "npm"
	default:
		return "npm"
	}
}

func DetectTestCommand(pm string, pkg packageJSON) string {
	if script := strings.TrimSpace(pkg.Scripts["test"]); script != "" && !isPlaceholderTest(script) {
		switch pm {
		case "bun":
			return "bun test"
		case "pnpm":
			return "pnpm test"
		case "yarn":
			return "yarn test"
		default:
			return "npm test"
		}
	}
	if hasDependency(pkg, "vitest") {
		return runCommand(pm, "vitest")
	}
	if hasDependency(pkg, "jest") {
		return runCommand(pm, "jest")
	}
	if pm == "bun" {
		return "bun test"
	}
	return ""
}

func DetectFramework(root string, pkg packageJSON) string {
	if exists(filepath.Join(root, "app", "api")) || exists(filepath.Join(root, "src", "app", "api")) || hasDependency(pkg, "next") {
		return "nextjs-app-router"
	}
	if hasDependency(pkg, "hono") {
		return "hono"
	}
	if hasDependency(pkg, "express") {
		return "express"
	}
	return "unknown"
}

func DiscoverNextAppRoutes(root string) ([]Route, error) {
	var routes []Route
	for _, base := range []string{filepath.Join(root, "app", "api"), filepath.Join(root, "src", "app", "api")} {
		if !exists(base) {
			continue
		}
		err := filepath.WalkDir(base, func(path string, entry os.DirEntry, err error) error {
			if err != nil || entry.IsDir() || filepath.Base(path) != "route.ts" {
				return err
			}
			body, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			methods := exportedMethods(string(body))
			routePath := routePathFromFile(base, path)
			for _, method := range methods {
				routes = append(routes, Route{Method: method, Path: routePath})
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	sort.Slice(routes, func(i, j int) bool {
		if routes[i].Path == routes[j].Path {
			return routes[i].Method < routes[j].Method
		}
		return routes[i].Path < routes[j].Path
	})
	return routes, nil
}

// expressRoutePattern matches Express/Hono route definitions like:
//   app.get("/api/users", ...)
//   router.post('/api/health', ...)
//   app.delete(`/api/items/:id`, ...)
var expressRoutePattern = regexp.MustCompile(
	`(?:app|router|server)\.(get|post|put|patch|delete)\s*\(\s*["'` + "`" + `](/[^"'` + "`" + `]*)["'` + "`" + `]`,
)

// DiscoverExpressRoutes scans source files for Express/Hono route patterns.
// It looks in common source directories (src/, routes/, lib/, and root-level .ts/.js files).
func DiscoverExpressRoutes(root string) ([]Route, error) {
	var routes []Route
	seen := map[string]bool{}

	scanDirs := []string{root}
	for _, sub := range []string{"src", "routes", "lib", "api"} {
		dir := filepath.Join(root, sub)
		if exists(dir) {
			scanDirs = append(scanDirs, dir)
		}
	}

	for _, dir := range scanDirs {
		_ = filepath.WalkDir(dir, func(path string, entry os.DirEntry, err error) error {
			if err != nil || entry.IsDir() {
				// Skip node_modules, .next, dist, and hidden directories.
				if entry != nil && entry.IsDir() {
					name := entry.Name()
					if name == "node_modules" || name == ".next" || name == "dist" || name == ".git" || strings.HasPrefix(name, ".") {
						return filepath.SkipDir
					}
				}
				return err
			}
			ext := filepath.Ext(path)
			if ext != ".ts" && ext != ".js" && ext != ".mts" && ext != ".mjs" {
				return nil
			}
			body, readErr := os.ReadFile(path)
			if readErr != nil {
				return nil
			}
			matches := expressRoutePattern.FindAllStringSubmatch(string(body), -1)
			for _, match := range matches {
				method := strings.ToUpper(match[1])
				routePath := match[2]
				key := method + " " + routePath
				if !seen[key] {
					seen[key] = true
					routes = append(routes, Route{Method: method, Path: routePath})
				}
			}
			return nil
		})
	}

	sort.Slice(routes, func(i, j int) bool {
		if routes[i].Path == routes[j].Path {
			return routes[i].Method < routes[j].Method
		}
		return routes[i].Path < routes[j].Path
	})
	return routes, nil
}

func readPackageJSON(root string) (packageJSON, error) {
	var pkg packageJSON
	data, err := os.ReadFile(filepath.Join(root, "package.json"))
	if err != nil {
		return pkg, err
	}
	if err := json.Unmarshal(data, &pkg); err != nil {
		return pkg, err
	}
	if pkg.Scripts == nil {
		pkg.Scripts = map[string]string{}
	}
	return pkg, nil
}

func exportedMethods(body string) []string {
	methods := []string{}
	for _, method := range []string{"GET", "POST", "PUT", "PATCH", "DELETE"} {
		if strings.Contains(body, "function "+method) || strings.Contains(body, "const "+method) || strings.Contains(body, "async function "+method) {
			methods = append(methods, method)
		}
	}
	return methods
}

func routePathFromFile(base, path string) string {
	rel, err := filepath.Rel(base, filepath.Dir(path))
	if err != nil || rel == "." {
		return "/api"
	}
	parts := strings.Split(filepath.ToSlash(rel), "/")
	out := []string{"api"}
	for _, part := range parts {
		if part == "" || strings.HasPrefix(part, "(") {
			continue
		}
		if strings.HasPrefix(part, "[") && strings.HasSuffix(part, "]") {
			part = ":" + strings.Trim(part, "[]")
		}
		out = append(out, part)
	}
	return "/" + strings.Join(out, "/")
}

func runCommand(pm, binary string) string {
	switch pm {
	case "bun":
		return "bunx " + binary
	case "pnpm":
		return "pnpm exec " + binary
	case "yarn":
		return "yarn " + binary
	default:
		return "npx " + binary
	}
}

func hasDependency(pkg packageJSON, name string) bool {
	_, ok := pkg.Dependencies[name]
	if ok {
		return true
	}
	_, ok = pkg.DevDeps[name]
	return ok
}

func isPlaceholderTest(script string) bool {
	lower := strings.ToLower(script)
	return strings.Contains(lower, "no test specified") || strings.Contains(lower, "exit 1")
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// W5: DiscoverRoutesFromTests parses test files (vitest/jest) to find route
// assertions like fetch("/api/users") or request(app).get("/api/health").
// This supplements static route discovery when test files reference routes
// that aren't discoverable from source alone.
func DiscoverRoutesFromTests(root string) []Route {
	var routes []Route
	seen := map[string]bool{}

	// Common test file locations.
	testDirs := []string{root}
	for _, sub := range []string{"tests", "test", "__tests__", "src", "spec"} {
		dir := filepath.Join(root, sub)
		if exists(dir) {
			testDirs = append(testDirs, dir)
		}
	}

	for _, dir := range testDirs {
		_ = filepath.WalkDir(dir, func(path string, entry os.DirEntry, err error) error {
			if err != nil || entry.IsDir() {
				if entry != nil && entry.IsDir() {
					name := entry.Name()
					if name == "node_modules" || name == ".next" || name == "dist" || name == ".git" {
						return filepath.SkipDir
					}
				}
				return err
			}
			// Only look at test files.
			if !isTestFile(path) {
				return nil
			}
			body, readErr := os.ReadFile(path)
			if readErr != nil {
				return nil
			}
			content := string(body)

			// Match patterns like:
			//   fetch("/api/users")
			//   fetch(`/api/users`)
			//   request(app).get("/api/health")
			//   .get("/api/health")
			//   .post("/api/login")
			for _, match := range testRoutePattern.FindAllStringSubmatch(content, -1) {
				method := strings.ToUpper(match[1])
				routePath := match[2]
				if method == "" || method == "FETCH" {
					method = "GET"
				}
				key := method + " " + routePath
				if !seen[key] && strings.HasPrefix(routePath, "/api") {
					seen[key] = true
					routes = append(routes, Route{Method: method, Path: routePath})
				}
			}
			return nil
		})
	}

	sort.Slice(routes, func(i, j int) bool {
		if routes[i].Path == routes[j].Path {
			return routes[i].Method < routes[j].Method
		}
		return routes[i].Path < routes[j].Path
	})
	return routes
}

// testRoutePattern matches common test patterns for API route assertions.
// Captures: (method, path)
// Examples:
//   .get("/api/users")  → GET, /api/users
//   .post('/api/login') → POST, /api/login
//   fetch("/api/health") → (empty), /api/health
//   fetch(`${baseUrl}/api/users`) → skipped (dynamic)
var testRoutePattern = regexp.MustCompile(
	`(?:\.|\b)(get|post|put|patch|delete|fetch)\s*\(\s*["'` + "`" + `](/api[^"'` + "`" + `\$]*)["'` + "`" + `]`,
)

// isTestFile returns true if the file looks like a test file.
func isTestFile(path string) bool {
	base := filepath.Base(path)
	ext := filepath.Ext(base)
	if ext != ".ts" && ext != ".js" && ext != ".tsx" && ext != ".jsx" && ext != ".mts" && ext != ".mjs" {
		return false
	}
	nameWithoutExt := strings.TrimSuffix(base, ext)
	return strings.HasSuffix(nameWithoutExt, ".test") ||
		strings.HasSuffix(nameWithoutExt, ".spec") ||
		strings.HasSuffix(nameWithoutExt, "_test") ||
		strings.HasSuffix(nameWithoutExt, "_spec")
}
