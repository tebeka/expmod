package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
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
	httpTimeout time.Duration
)

const (
	tokenKey = "GITHUB_TOKEN" // #nosec G101
)

var extraHelp = `
If %s is found in the environment, it will be use to access GitHub API.
"Human" GitHub URLs (e.g. https://github.com/tebeka/expmod/blob/main/go.mod) will be redirected to raw content.
`

func main() {
	exe := path.Base(os.Args[0])
	flag.BoolVar(&showVersion, "version", false, "show version and exit")
	flag.DurationVar(&httpTimeout, "timeout", 3*time.Second, "HTTP timeout")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: %s [options] [file or URL]\nOptions:\n", exe)
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, extraHelp, tokenKey)
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

	var r io.ReadCloser = os.Stdin
	if flag.NArg() == 1 {
		uri := flag.Arg(0)

		var err error
		if strings.HasPrefix(uri, "https://") || strings.HasPrefix(uri, "http://") {
			r, err = openURL(uri)
		} else {
			r, err = os.Open(flag.Arg(0))
		}

		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %s", err)
			os.Exit(1)
		}
		defer r.Close()
	}

	cache, err := loadCache()
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			slog.Warn("can't load cache", "error", err)
		}
		cache = make(map[string]string)
	}

	if err := pkgsInfo(r, cache); err != nil {
		fmt.Fprintf(os.Stderr, "error: too many arguments\n")
		os.Exit(1)
	}

	if err := saveCache(cache); err != nil {
		slog.Warn("can't save cache", "error", err)
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
		if require.Indirect {
			continue
		}

		pkg := require.Mod.Path
		pkgName := pkg // for proxy
		if !strings.HasPrefix(pkg, "github.com") {
			var err error
			ctx, cancel := context.WithTimeout(context.Background(), httpTimeout)
			defer cancel()

			pkg, err = proxyRepo(ctx, pkg)
			if err != nil {
				// TODO: log?
				continue
			}
		}

		owner, repo := repoInfo(pkg)
		if owner == "" || repo == "" {
			slog.Warn("can't get info", "package", pkg)
			continue
		}

		key := fmt.Sprintf("%s/%s", owner, repo)
		desc, ok := cache[key]
		if !ok {
			var err error
			desc, err = repoDesc(owner, repo)
			if err != nil {
				slog.Error("can't get description", "package", pkgName, "repo", pkg, "error", err)
				continue
			}
			cache[key] = desc
		}

		fmt.Printf("%s %s:\n\t%s\n", pkgName, require.Mod.Version, desc)
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

	token := os.Getenv(tokenKey)
	if token != "" {
		req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", token))
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

/*
https://github.com/nxadm/tail/blob/master/go.mod ->
https://raw.githubusercontent.com/nxadm/tail/master/go.mod
*/
func githubRawURL(ghURL string) (string, error) {
	u, err := url.Parse(ghURL)
	if err != nil {
		return "", err
	}
	// https://github.com/nxadm/tail/blob/master/go.mod
	//                 0    1     2    3    4      5
	fields := strings.Split(u.Path, "/")
	if len(fields) < 6 {
		return "", fmt.Errorf("%q too short", ghURL)
	}
	owner, repo, branch, file := fields[1], fields[2], fields[4], fields[5]
	u.Host = "raw.githubusercontent.com"
	path, err := url.JoinPath(owner, repo, branch, file)
	if err != nil {
		return "", fmt.Errorf("can't construct URL - %w", err)
	}

	u.Path = path
	return u.String(), nil
}

func openURL(url string) (io.ReadCloser, error) {
	if strings.Contains(url, "github.com") {
		var err error
		url, err = githubRawURL(url)
		if err != nil {
			return nil, fmt.Errorf("%q: bad URL- %w", url, err)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), httpTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("%q: bad URL- %w", url, err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%q: can't get- %w", url, err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%q: bad status - %s", url, resp.Status)
	}

	return resp.Body, nil
}
