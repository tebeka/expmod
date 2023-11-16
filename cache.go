package main

import (
	"encoding/gob"
	"fmt"
	"os"
	"os/user"
	"path"
)

func defaultCacheFile() (string, error) {
	u, err := user.Current()
	if err != nil {
		return "", err
	}

	return path.Join(u.HomeDir, ".local", "cache", "expmod", "cache.gob"), nil
}

const cacheEnvKey = "EXPMOD_CACHE"

func cacheFileName() (string, error) {
	if p := os.Getenv(cacheEnvKey); p != "" {
		return p, nil
	}

	return defaultCacheFile()
}

func loadCache() (map[string]string, error) {
	fileName, err := cacheFileName()
	if err != nil {
		return nil, err
	}

	file, err := os.Open(fileName)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var cache map[string]string
	if err := gob.NewDecoder(file).Decode(&cache); err != nil {
		return nil, fmt.Errorf("can't load %q - %w", fileName, err)
	}

	return cache, nil
}

func saveCache(cache map[string]string) error {
	fileName, err := cacheFileName()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(path.Dir(fileName), 0750); err != nil {
		return err
	}

	file, err := os.Create(fileName)
	if err != nil {
		return err
	}
	defer file.Close()

	if err := gob.NewEncoder(file).Encode(cache); err != nil {
		return err
	}
	return nil
}
