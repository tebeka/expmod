package main

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

const (
	zapRepo = "https://github.com/uber-go/zap"
)

func Test_parsePkgHTML(t *testing.T) {
	file, err := os.Open("testdata/pkg-zap.html")
	require.NoError(t, err, "read HTML")
	defer file.Close()

	repo, err := parsePkgHTML(file)
	require.NoError(t, err, "parse HTML")
	require.Equal(t, zapRepo, repo)
}

func Test_repoFromPkg(t *testing.T) {
	if os.Getenv("CI") != "" {
		t.Skip("In CI")
	}

	ctx, cancel := testCtx(t)
	defer cancel()

	repo, err := repoFromPkg(ctx, "go.uber.org/zap")
	require.NoError(t, err, "parse HTML")
	require.Equal(t, zapRepo, repo)
}
