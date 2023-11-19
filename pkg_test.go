package main

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_parsePkgHTML(t *testing.T) {
	file, err := os.Open("testdata/pkg-zap.html")
	require.NoError(t, err, "read HTML")
	defer file.Close()

	repo, err := parsePkgHTML(file)
	require.NoError(t, err, "parse HTML")
	require.Equal(t, "https://github.com/uber-go/zap", repo)
}
