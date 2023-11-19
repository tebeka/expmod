package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"golang.org/x/mod/modfile"
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
			require.Equal(t, tc.owner, owner, "owner")
			require.Equal(t, tc.repo, repo, "repo")
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
	require.NoError(t, err, "API")
	require.Equal(t, expected, desc, "description")
}

func Test_ignored(t *testing.T) {
	const gomod = `
module test
go 1.20
require (
	cuelang.org/go v0.4.3
	github.com/cenkalti/backoff/v4 v4.1.2
	github.com/benbjohnson/clock v1.3.3 // indirect
)
`
	f, err := modfile.ParseLax("go.mod", []byte(gomod), nil)
	require.NoError(t, err)

	ignores := []bool{true, false, true}
	for i, req := range f.Require {
		t.Run(req.Mod.Path, func(t *testing.T) {
			v := ignored(req)
			require.Equal(t, ignores[i], v)
		})
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
	t.Logf("exe: %q", exe)
	ctx, cancel := testCtx(t)
	defer cancel()

	err := exec.CommandContext(ctx, "go", "build", "-o", exe).Run()
	require.NoError(t, err, "build")
	return exe
}

var testMod = "testdata/go.mod"
var exeExpected = `github.com/sahilm/fuzzy v0.1.0:
	Go library that provides fuzzy string matching optimized for filenames and code symbols in the style of Sublime Text, VSCode, IntelliJ IDEA et al.
github.com/stretchr/testify v1.8.4:
	A toolkit with common assertions and mocks that plays nicely with the standard library
`

func TestExe(t *testing.T) {
	exe := build(t)

	ctx, cancel := testCtx(t)
	defer cancel()

	var buf bytes.Buffer
	cmd := exec.CommandContext(ctx, exe, testMod)
	cmd.Stdout = &buf
	err := cmd.Run()

	require.NoError(t, err, "run")
	require.Equal(t, exeExpected, buf.String())
}

func TestExeStdin(t *testing.T) {
	exe := build(t)

	ctx, cancel := testCtx(t)
	defer cancel()

	file, err := os.Open(testMod)
	require.NoError(t, err, "open mod")
	defer file.Close()

	var buf bytes.Buffer
	cmd := exec.CommandContext(ctx, exe)
	cmd.Stdin = file
	cmd.Stdout = &buf

	err = cmd.Run()
	require.NoError(t, err, "run")
	require.Equal(t, exeExpected, buf.String())
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
			require.NoError(t, err, "run")
			require.Contains(t, string(out), tc.fragment)
		})
	}
}
