package main

import (
	"testing"

	"github.com/stretchr/testify/require"
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
	owner, repo := "pkg", "errors"
	expected := "Simple error handling primitives" // FIXME: brittle

	desc, err := repoDesc(owner, repo)
	require.NoError(t, err, "API")
	require.Equal(t, expected, desc, "description")
}

var ignoredCases = []struct {
	line    string
	ignored bool
}{
	{"cuelang.org/go v0.4.3", true},
	{"github.com/cenkalti/backoff/v4 v4.1.2", false},
	{"github.com/benbjohnson/clock v1.3.3 // indirect", true},
	{"", true},
}

func Test_ignored(t *testing.T) {
	for _, tc := range ignoredCases {
		t.Run(tc.line, func(t *testing.T) {
			v := ignored(tc.line)
			require.Equal(t, tc.ignored, v)
		})
	}
}
