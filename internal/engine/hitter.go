package engine

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Bharath-code/regressguard/internal/config"
)

const (
	defaultRouteTimeout  = 10 * time.Second
	serverProbeTimeout   = 500 * time.Millisecond
	maxBodyBytes         = 1 << 20 // 1 MB
)

// RouteResult holds the outcome of hitting a single route.
type RouteResult struct {
	Method     string
	Path       string
	Status     int
	SchemaHash string
	// NormalizedSchema is the JSON-encoded normalized type shape.
	// Populated by HitRoutes so the snapshot engine can store it for field-level diff.
	NormalizedSchema []byte
	MS               int64
	Skipped          bool
	SkipReason       string
	// Body is only populated when verbose mode is active.
	Body []byte
}

// HitOptions configures the route hitter.
type HitOptions struct {
	ServerURL    string
	Auth         config.Auth
	IgnoreFields []string
	Timeout      time.Duration
	Verbose      bool
	// HTTPClient allows injection of a custom client (useful in tests).
	HTTPClient *http.Client
}

// ServerReachable probes the server URL with a short timeout.
// Returns false within 500ms if the server is not responding.
// This is used by rg check to fail fast instead of timing out per-route.
func ServerReachable(serverURL string) bool {
	client := &http.Client{Timeout: serverProbeTimeout}
	// Probe the root path — we just need a TCP connection, not a 200.
	req, err := http.NewRequestWithContext(context.Background(), "GET", strings.TrimRight(serverURL, "/")+"/", nil)
	if err != nil {
		return false
	}
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	resp.Body.Close()
	return true
}

// HitRoutes calls each non-skipped route in the config and returns results.
// Routes that require a body (POST/PUT/PATCH with no body configured) are
// skipped gracefully. Progress lines are written to progressWriter when set.
func HitRoutes(routes []config.Route, opts HitOptions, progressWriter io.Writer) []RouteResult {
	client := opts.HTTPClient
	if client == nil {
		timeout := opts.Timeout
		if timeout == 0 {
			timeout = defaultRouteTimeout
		}
		client = &http.Client{Timeout: timeout}
	}

	results := make([]RouteResult, 0, len(routes))
	for _, route := range routes {
		if route.Skip {
			results = append(results, RouteResult{
				Method:     route.Method,
				Path:       route.Path,
				Skipped:    true,
				SkipReason: "marked skip in config",
			})
			if progressWriter != nil {
				_, _ = fmt.Fprintf(progressWriter, "  - SKIPPED %s %s (config)\n", route.Method, route.Path)
			}
			continue
		}

		// Skip routes that require a body and have no body configured.
		if requiresBody(route.Method) {
			results = append(results, RouteResult{
				Method:     route.Method,
				Path:       route.Path,
				Skipped:    true,
				SkipReason: "body params required — add to skip list or provide body in config",
			})
			if progressWriter != nil {
				_, _ = fmt.Fprintf(progressWriter, "  - SKIPPED %s %s (body required)\n", route.Method, route.Path)
			}
			continue
		}

		result := hitRoute(client, route, opts)
		results = append(results, result)

		if progressWriter != nil {
			if result.Skipped {
				_, _ = fmt.Fprintf(progressWriter, "  - SKIPPED %s %s (%s)\n", route.Method, route.Path, result.SkipReason)
			} else {
				_, _ = fmt.Fprintf(progressWriter, "  > %s %s  %d  %dms\n", route.Method, route.Path, result.Status, result.MS)
			}
		}
	}
	return results
}

func hitRoute(client *http.Client, route config.Route, opts HitOptions) RouteResult {
	url := buildURL(opts.ServerURL, route.Path)

	ctx, cancel := context.WithTimeout(context.Background(), defaultRouteTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, route.Method, url, nil)
	if err != nil {
		return RouteResult{
			Method:     route.Method,
			Path:       route.Path,
			Skipped:    true,
			SkipReason: "could not build request: " + err.Error(),
		}
	}

	applyAuth(req, opts.Auth)

	start := time.Now()
	resp, err := client.Do(req)
	elapsed := time.Since(start).Milliseconds()

	if err != nil {
		return RouteResult{
			Method:     route.Method,
			Path:       route.Path,
			Skipped:    true,
			SkipReason: "request failed: " + err.Error(),
		}
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))

	schemaHash, shapeJSON := NormalizeAndHashWithShape(body, opts.IgnoreFields)

	result := RouteResult{
		Method:           route.Method,
		Path:             route.Path,
		Status:           resp.StatusCode,
		SchemaHash:       schemaHash,
		NormalizedSchema: shapeJSON,
		MS:               elapsed,
	}
	if opts.Verbose {
		result.Body = body
	}
	return result
}

func applyAuth(req *http.Request, auth config.Auth) {
	switch strings.ToLower(auth.Mode) {
	case "bearer", "token":
		if auth.TestToken != "" {
			headerName := auth.HeaderName
			if headerName == "" {
				headerName = "Authorization"
			}
			prefix := auth.Prefix
			if prefix == "" {
				prefix = "Bearer"
			}
			req.Header.Set(headerName, prefix+" "+auth.TestToken)
		}
	case "cookie":
		if auth.Cookie != "" {
			req.Header.Set("Cookie", auth.Cookie)
		}
	}
}

func buildURL(serverURL, path string) string {
	base := strings.TrimRight(serverURL, "/")
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return base + path
}

func requiresBody(method string) bool {
	switch strings.ToUpper(method) {
	case "POST", "PUT", "PATCH":
		return true
	}
	return false
}
