package main

import (
	"path"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_cacheFile(t *testing.T) {
	fileName, err := cacheFileName()
	require.NoError(t, err)
	require.True(t, strings.HasSuffix(fileName, ".local/cache/expmod/cache.gob"))

	tmpCache := "/tmp/expmod-cached.gob"
	t.Setenv(cacheEnvKey, tmpCache)
	fileName, err = cacheFileName()
	require.NoError(t, err)
	require.Equal(t, tmpCache, fileName)
}

func TestCache(t *testing.T) {
	tmpDir := t.TempDir()
	cacheFile := path.Join(tmpDir, "cache.gob")
	t.Setenv(cacheEnvKey, cacheFile)

	cache := map[string]string{
		"a": "b",
		"b": "c",
	}
	err := saveCache(cache)
	require.NoError(t, err, "save")

	loaded, err := loadCache()
	require.NoError(t, err, "load")
	require.Equal(t, cache, loaded)

}
