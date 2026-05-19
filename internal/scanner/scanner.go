package scanner

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
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
	routes, _ := DiscoverNextAppRoutes(root)
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
