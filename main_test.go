package main

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
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
	if os.Getenv("CI") != "" {
		t.Skip("In CI")
	}

	owner, repo := "pkg", "errors"
	expected := "Simple error handling primitives" // FIXME: brittle

	ctx, cancel := testCtx(t)
	defer cancel()

	desc, err := repoDesc(ctx, owner, repo)
	if err != nil {
		t.Fatalf("API: %v", err)
	}

	if desc != expected {
		t.Fatalf("description: expected %q, got %q", expected, desc)
	}
}

func Test_repoDescFallbackToReadme(t *testing.T) {
	if os.Getenv("CI") != "" {
		t.Skip("In CI")
	}

	// bmizerany/pat has no GitHub description, so we fall back to README
	owner, repo := "bmizerany", "pat"
	ctx, cancel := testCtx(t)
	defer cancel()

	desc, err := repoDesc(ctx, owner, repo)
	if err != nil {
		t.Fatalf("repoDesc: %v", err)
	}
	if desc == "" {
		t.Fatal("expected non-empty description from README fallback")
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
			cmd := exec.CommandContext(ctx, exe, testMod)
			cmd.Stdout = &buf
			err := cmd.Run()

			if err != nil {
				t.Fatalf("run: %v", err)
			}

			if buf.String() != exeExpected {
				t.Fatalf("expected %q, got %q", exeExpected, buf.String())
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
	var mt mockTripper
	http.DefaultClient.Transport = &mt
	t.Cleanup(func() {
		http.DefaultClient.Transport = oldTransport
	})

	ctx, cancel := testCtx(t)
	defer cancel()
	repoDesc(ctx, "tebeka", "expmod") // Should err, we don't care - it's a mock

	if mt.token != token {
		t.Fatalf("expected token %q, got %q", token, mt.token)
	}
}

func Test_githubRawURL(t *testing.T) {
	// TODO: More tests
	url := "https://github.com/nxadm/tail/blob/master/go.mod"
	expected := "https://raw.githubusercontent.com/nxadm/tail/master/go.mod"

	out, err := githubRawURL(url)
	if err != nil {
		t.Fatalf("githubRawURL: %v", err)
	}

	if out != expected {
		t.Fatalf("expected %q, got %q", expected, out)
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
