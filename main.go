package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"
	"time"

	"golang.org/x/mod/modfile"
)

var (
	version, commit = "???", "???"

	showVersion bool
)

func main() {
	exe := path.Base(os.Args[0])
	flag.BoolVar(&showVersion, "version", false, "show version and exit")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: %s [options] [file]\nOptions:\n", exe)
		flag.PrintDefaults()
	}
	flag.Parse()

	if showVersion {
		fmt.Printf("%s version %s (commit %s)\n", exe, version, commit)
		os.Exit(0)
	}

	if flag.NArg() > 1 {
		fmt.Fprintf(os.Stderr, "error: too many arguments\n")
		os.Exit(1)
	}

	var r io.Reader = os.Stdin
	if flag.NArg() == 1 {
		file, err := os.Open(flag.Arg(0))
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %s", err)
			os.Exit(1)
		}
		defer file.Close()
		r = file
	}

	cache, err := loadCache()
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			log.Printf("warning: can't load cache: %s", err)
		}
		cache = make(map[string]string)
	}

	if err := pkgsInfo(r, cache); err != nil {
		log.Fatalf("error: %s", err)
	}

	if err := saveCache(cache); err != nil {
		log.Printf("warning: can't save cache: %s", err)
	}
}

func pkgsInfo(r io.Reader, cache map[string]string) error {
	const maxSize = 16 * (1 << 20) // go.mod files are limited to 16 MiB
	data, err := io.ReadAll(io.LimitReader(r, maxSize))
	if err != nil {
		return err
	}

	f, err := modfile.ParseLax("go.mod", data, nil)
	if err != nil {
		return err
	}

	for _, require := range f.Require {
		if ignored(require) {
			continue
		}
		line := require.Mod.Path

		owner, repo := repoInfo(line)
		if owner == "" || repo == "" {
			fmt.Fprintf(os.Stderr, "error: %s\n", line)
			continue
		}

		key := fmt.Sprintf("%s/%s", owner, repo)
		desc, ok := cache[key]
		if !ok {
			var err error
			desc, err = repoDesc(owner, repo)
			if err != nil {
				fmt.Fprintf(os.Stderr, "error: %s - %s\n", line, err)
				continue
			}
			cache[key] = desc
		}

		fmt.Printf("%s %s:\n\t%s\n", line, require.Mod.Version, desc)
	}

	return nil
}

func repoDesc(owner, repo string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	url := fmt.Sprintf("https://api.github.com/repos/%s/%s", url.PathEscape(owner), url.PathEscape(repo))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("%q: %s", url, resp.Status)
	}

	var reply struct {
		Description string
	}

	if err := json.NewDecoder(resp.Body).Decode(&reply); err != nil {
		return "", fmt.Errorf("%q: can't decode JSON - %w", url, err)
	}

	return reply.Description, nil
}

// repoInfo extract repository information from line.
// e.g. "github.com/go-redis/redis/v8 v8.11.5" -> "go-redis", "redis"
func repoInfo(line string) (string, string) {
	fields := strings.Split(line, "/")
	if len(fields) < 3 {
		return "", ""
	}
	owner := fields[1]
	repo, _, _ := strings.Cut(fields[2], " ")
	return owner, repo
}

func ignored(require *modfile.Require) bool {
	if !strings.HasPrefix(require.Mod.Path, "github.com") {
		return true
	}

	if require.Indirect {
		return true
	}

	return false
}
