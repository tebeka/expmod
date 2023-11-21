package main

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

var htmlCases = []struct {
	file string
	repo string
}{
	{"testdata/get-1.html", "github.com/go-yaml/yaml"},
	{"testdata/get-2.html", "github.com/uber-go/zap"},
}

func Test_parseProxyHTML(t *testing.T) {
	for _, tc := range htmlCases {
		t.Run(tc.file, func(t *testing.T) {
			file, err := os.Open(tc.file)
			require.NoError(t, err, "open")
			defer file.Close()

			repo, err := parseProxyHTML(file)
			require.NoError(t, err, "parse HTML")
			require.Equal(t, tc.repo, repo)
		})
	}
}

func Test_proxyRepo(t *testing.T) {
	if os.Getenv("CI") != "" {
		t.Skip("in CI")
	}

	pkg := "gopkg.in/yaml.v3"
	ctx, cancel := testCtx(t)
	defer cancel()
	repo, err := proxyRepo(ctx, pkg)
	require.NoError(t, err)
	require.Equal(t, "github.com/go-yaml/yaml", repo)
}
