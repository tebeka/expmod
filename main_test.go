package main

import (
	"os"
	"testing"

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
