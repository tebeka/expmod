package main

import (
	"path"
	"strings"
	"testing"
)

func Test_cacheFile(t *testing.T) {
	fileName, err := cacheFileName()
	if err != nil {
		t.Fatalf("cacheFileName: %v", err)
	}
	if !strings.HasSuffix(fileName, ".local/cache/expmod/cache.gob") {
		t.Fatalf("expected filename to end with .local/cache/expmod/cache.gob, got %q", fileName)
	}

	tmpCache := "/tmp/expmod-cached.gob"
	t.Setenv(cacheEnvKey, tmpCache)
	fileName, err = cacheFileName()
	if err != nil {
		t.Fatalf("cacheFileName: %v", err)
	}
	if fileName != tmpCache {
		t.Fatalf("expected %q, got %q", tmpCache, fileName)
	}
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
	if err != nil {
		t.Fatalf("save: %v", err)
	}

	loaded, err := loadCache()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	for k, v := range cache {
		if loaded[k] != v {
			t.Fatalf("cache mismatch for key %q: expected %q, got %q", k, v, loaded[k])
		}
	}
	for k, v := range loaded {
		if cache[k] != v {
			t.Fatalf("loaded mismatch for key %q: expected %q, got %q", k, v, cache[k])
		}
	}

}
