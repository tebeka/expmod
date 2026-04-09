package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path"
	"strings"
	"testing"
	"time"
)

var infoCases = []struct {
	line  string
	owner string
	repo  string
}{
	{
		"github.com/sahilm/fuzzy v0.1.0",
		"sahilm",
		"fuzzy",
	},
	{
		"github.com/cenkalti/backoff/v4 v4.1.2",
		"cenkalti",
		"backoff",
	},
	{
		"Go forward",
		"",
		"",
	},
}

func Test_repoInfo(t *testing.T) {
	for _, tc := range infoCases {
		t.Run(tc.line, func(t *testing.T) {
			owner, repo := repoInfo(tc.line)
			if owner != tc.owner {
				t.Fatalf("owner: expected %q, got %q", tc.owner, owner)
			}
			if repo != tc.repo {
				t.Fatalf("repo: expected %q, got %q", tc.repo, repo)
			}
		})
	}

}

func Test_repoDesc(t *testing.T) {
	restore := setupGitHubHTTP(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/pkg/errors" {
			http.NotFound(w, r)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"description":"Simple error handling primitives"}`)
	})
	defer restore()

	ctx, cancel := testCtx(t)
	defer cancel()

	desc, err := repoDesc(ctx, "pkg", "errors")
	if err != nil {
		t.Fatalf("API: %v", err)
	}

	expected := "Simple error handling primitives"
	if desc != expected {
		t.Fatalf("description: expected %q, got %q", expected, desc)
	}
}

func Test_repoDescFallbackToReadme(t *testing.T) {
	restore := setupGitHubHTTP(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/bmizerany/pat":
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"description":""}`)
		case "/bmizerany/pat/HEAD/README.md":
			_, _ = io.WriteString(w, "# Pat\n\nrouter")
		default:
			http.NotFound(w, r)
		}
	})
	defer restore()

	ctx, cancel := testCtx(t)
	defer cancel()

	desc, err := repoDesc(ctx, "bmizerany", "pat")
	if err != nil {
		t.Fatalf("repoDesc: %v", err)
	}

	if desc != "Pat" {
		t.Fatalf("expected README description, got %q", desc)
	}
}

func Test_repoDescStatusError(t *testing.T) {
	restore := setupGitHubHTTP(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "rate limited", http.StatusTooManyRequests)
	})
	defer restore()

	ctx, cancel := testCtx(t)
	defer cancel()

	_, err := repoDesc(ctx, "pkg", "errors")
	if err == nil {
		t.Fatal("expected error")
	}

	if !strings.Contains(err.Error(), "429") {
		t.Fatalf("expected status in error, got %v", err)
	}
}

func testCtx(t *testing.T) (context.Context, context.CancelFunc) {
	deadline, ok := t.Deadline()
	if ok {
		return context.WithDeadline(context.Background(), deadline)
	}

	return context.WithTimeout(context.Background(), 3*time.Second)
}

func build(t *testing.T) string {
	exe := path.Join(t.TempDir(), "expmod")
	ctx, cancel := testCtx(t)
	defer cancel()

	err := exec.CommandContext(ctx, "go", "build", "-o", exe).Run()
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	return exe
}

var testMod = "testdata/go.mod"
var exeExpected = `github.com/sahilm/fuzzy v0.1.0:
	Go library that provides fuzzy string matching optimized for filenames and code symbols in the style of Sublime Text, VSCode, IntelliJ IDEA et al.
github.com/stretchr/testify v1.8.4:
	A toolkit with common assertions and mocks that plays nicely with the standard library
`
var proxyMod = "testdata/proxy.mod"
var proxyExpected = `gopkg.in/yaml.v3 v3.0.1:
	YAML support for the Go language.
`

var exeCases = []struct {
	file   string
	output string
}{
	{testMod, exeExpected},
	{proxyMod, proxyExpected},
}

func TestExe(t *testing.T) {
	exe := build(t)

	for _, tc := range exeCases {
		t.Run(tc.file, func(t *testing.T) {
			ctx, cancel := testCtx(t)
			defer cancel()

			var buf bytes.Buffer
			cmd := exec.CommandContext(ctx, exe, tc.file)
			cmd.Stdout = &buf
			err := cmd.Run()

			if err != nil {
				t.Fatalf("run: %v", err)
			}

			if buf.String() != tc.output {
				t.Fatalf("expected %q, got %q", tc.output, buf.String())
			}
		})
	}
}

func TestExeStdin(t *testing.T) {
	exe := build(t)

	ctx, cancel := testCtx(t)
	defer cancel()

	file, err := os.Open(testMod)
	if err != nil {
		t.Fatalf("open mod: %v", err)
	}
	defer file.Close()

	var buf bytes.Buffer
	cmd := exec.CommandContext(ctx, exe)
	cmd.Stdin = file
	cmd.Stdout = &buf

	err = cmd.Run()
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if buf.String() != exeExpected {
		t.Fatalf("expected %q, got %q", exeExpected, buf.String())
	}
}

var flagCases = []struct {
	flag     string
	fragment string
}{
	{"-version", "version"},
	{"-help", "usage"},
	{"-clear-cache", "cache cleared"},
}

func TestExeFlags(t *testing.T) {
	exe := build(t)

	for _, tc := range flagCases {
		t.Run(tc.flag, func(t *testing.T) {
			ctx, cancel := testCtx(t)
			defer cancel()

			cmd := exec.CommandContext(ctx, exe, tc.flag)
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("run: %v", err)
			}
			if !strings.Contains(string(out), tc.fragment) {
				t.Fatalf("expected output to contain %q, got %q", tc.fragment, string(out))
			}
		})
	}
}

type mockTripper struct {
	token string
}

func (t *mockTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	auth := req.Header.Get("Authorization")
	i := len("Bearer ")
	if len(auth) > i {
		t.token = auth[i:]
	}

	return nil, fmt.Errorf("oopsie")
}

func TestGHToken(t *testing.T) {
	token := "s3cr3t"
	t.Setenv(tokenKey, token)

	oldTransport := http.DefaultClient.Transport
	oldClient := httpClient
	var mt mockTripper
	http.DefaultClient.Transport = &mt
	httpClient = &http.Client{Transport: &mt}
	t.Cleanup(func() {
		http.DefaultClient.Transport = oldTransport
		httpClient = oldClient
	})

	ctx, cancel := testCtx(t)
	defer cancel()
	repoDesc(ctx, "tebeka", "expmod") // Should err, we don't care - it's a mock

	if mt.token != token {
		t.Fatalf("expected token %q, got %q", token, mt.token)
	}
}

func Test_githubRawURL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		want    string
		wantErr string
	}{
		{
			name: "top-level file",
			url:  "https://github.com/nxadm/tail/blob/master/go.mod",
			want: "https://raw.githubusercontent.com/nxadm/tail/master/go.mod",
		},
		{
			name: "nested file",
			url:  "https://github.com/owner/repo/blob/main/sub/dir/go.mod",
			want: "https://raw.githubusercontent.com/owner/repo/main/sub/dir/go.mod",
		},
		{
			name:    "non-blob URL",
			url:     "https://github.com/owner/repo/tree/main/go.mod",
			wantErr: "expected /blob/ path",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			out, err := githubRawURL(tc.url)
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("expected error containing %q, got %v", tc.wantErr, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("githubRawURL: %v", err)
			}

			if out != tc.want {
				t.Fatalf("expected %q, got %q", tc.want, out)
			}
		})
	}
}

func setupGitHubHTTP(t *testing.T, handler http.HandlerFunc) func() {
	t.Helper()

	ts := httptest.NewServer(handler)
	oldClient := httpClient
	oldAPIBase := githubAPIBase
	oldRawBase := githubRawBase

	httpClient = ts.Client()
	githubAPIBase = ts.URL
	githubRawBase = ts.URL

	return func() {
		httpClient = oldClient
		githubAPIBase = oldAPIBase
		githubRawBase = oldRawBase
		ts.Close()
	}
}

func TestClearCache(t *testing.T) {
	exe := build(t)
	tmpDir := t.TempDir()
	cacheFile := path.Join(tmpDir, "cache.gob")

	// Set cache location
	t.Setenv("EXPMOD_CACHE", cacheFile)

	// Create a cache file with some data using saveCache
	cache := map[string]string{"golang/go": "The Go programming language"}
	if err := saveCache(cache); err != nil {
		t.Fatalf("saveCache: %v", err)
	}

	// Verify cache file exists and contains data
	if _, err := os.Stat(cacheFile); err != nil {
		t.Fatalf("cache file not created: %v", err)
	}

	// Run with --clear-cache
	ctx, cancel := testCtx(t)
	defer cancel()

	cmd := exec.CommandContext(ctx, exe, "-clear-cache")
	cmd.Env = append(os.Environ(), "EXPMOD_CACHE="+cacheFile)
	out, err := cmd.CombinedOutput()

	if err != nil {
		t.Fatalf("run with --clear-cache: %v", err)
	}

	// Verify output
	if !strings.Contains(string(out), "cache cleared") {
		t.Fatalf("expected output to contain 'cache cleared', got %q", string(out))
	}

	// Verify cache file still exists but is now empty using loadCache
	loadedCache, err := loadCache()
	if err != nil {
		t.Fatalf("loadCache: %v", err)
	}

	if len(loadedCache) != 0 {
		t.Fatalf("expected empty cache, got %v", loadedCache)
	}
}

func TestPackagesSorted(t *testing.T) {
	exe := build(t)

	ctx, cancel := testCtx(t)
	defer cancel()

	var buf bytes.Buffer
	cmd := exec.CommandContext(ctx, exe, "testdata/unsorted.mod")
	cmd.Stdout = &buf

	err := cmd.Run()
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	output := buf.String()

	// Find positions of packages in output
	fuzzyPos := strings.Index(output, "github.com/sahilm/fuzzy")
	testifyPos := strings.Index(output, "github.com/stretchr/testify")

	if fuzzyPos == -1 {
		t.Fatalf("expected to find github.com/sahilm/fuzzy in output: %s", output)
	}

	if testifyPos == -1 {
		t.Fatalf("expected to find github.com/stretchr/testify in output: %s", output)
	}

	if fuzzyPos >= testifyPos {
		t.Fatalf("packages not in alphabetical order. fuzzy at %d, testify at %d. output:\n%s", fuzzyPos, testifyPos, output)
	}
}
