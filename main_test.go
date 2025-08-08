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

	desc, err := repoDesc(owner, repo)
	if err != nil {
		t.Fatalf("API: %v", err)
	}
	if desc != expected {
		t.Fatalf("description: expected %q, got %q", expected, desc)
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

	repoDesc("tebeka", "expmod")
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
