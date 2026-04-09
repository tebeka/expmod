package main

import (
	"bufio"
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
	"runtime/debug"
	"sort"
	"strings"
	"time"

	"github.com/mattn/go-isatty"
	"golang.org/x/mod/modfile"
)

var (
	showVersion bool
	clearCache  bool
	httpTimeout = 30 * time.Second
	repoName    string
	serveAddr   string
	httpClient  = http.DefaultClient
)

// PkgInfo holds the info for a single dependency.
type PkgInfo struct {
	Name    string
	Version string
	Desc    string
	URL     string
}

type repoCache interface {
	Get(key string) (string, bool)
	Set(key, value string)
}

type mapCache struct{ m map[string]string }

func (c mapCache) Get(key string) (string, bool) { v, ok := c.m[key]; return v, ok }
func (c mapCache) Set(key, value string)         { c.m[key] = value }

const (
	tokenKey = "GITHUB_TOKEN" // #nosec G704 G101
)

var extraHelp = `
If %s is found in the environment, it will be used to access GitHub API.
"Human" GitHub URLs (e.g. https://github.com/tebeka/expmod/blob/main/go.mod) will be redirected to raw content.
`

var githubAPIBase = "https://api.github.com"

func main() {
	exe := path.Base(os.Args[0])
	flag.BoolVar(&showVersion, "version", false, "show version and exit")
	flag.BoolVar(&clearCache, "clear-cache", false, "clear the cache and exit")
	flag.DurationVar(&httpTimeout, "timeout", httpTimeout, "HTTP timeout")
	flag.StringVar(&repoName, "repo", "", "GitHub repository name")
	flag.StringVar(&serveAddr, "serve", "", "start web server on host:port")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: %s [options] [file or URL]\nOptions:\n", exe)
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, extraHelp, tokenKey)
	}
	flag.Parse()

	if showVersion {
		version := buildVersion()
		fmt.Printf("%s version %s\n", exe, version)
		os.Exit(0)
	}

	if serveAddr != "" {
		serve(serveAddr)
		return
	}

	if clearCache {
		if err := saveCache(make(map[string]string)); err != nil {
			fmt.Fprintf(os.Stderr, "error: can't clear cache - %s\n", err)
			os.Exit(1)
		}

		fmt.Println("cache cleared")
		os.Exit(0)
	}

	if flag.NArg() > 1 {
		fmt.Fprintf(os.Stderr, "error: too many arguments\n")
		os.Exit(1)
	}

	if flag.NArg() == 1 && repoName != "" {
		fmt.Fprintf(os.Stderr, "error: both repo & file/URL provided\n")
		os.Exit(1)
	}

	var r io.ReadCloser = os.Stdin
	if flag.NArg() == 1 || repoName != "" {
		var uri string
		if repoName != "" {
			uri = fmt.Sprintf("%s/%s/HEAD/go.mod", githubRawBase, repoName)
		} else {
			uri = flag.Arg(0)
		}

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

	pkgs, err := pkgsInfo(r, mapCache{m: cache})
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %s\n", err)
		os.Exit(1)
	}
	for _, p := range pkgs {
		displayInfo(p.Name, p.Version, p.Desc)
	}

	if err := saveCache(cache); err != nil {
		slog.Warn("can't save cache", "error", err)
	}
}

func pkgsInfo(r io.Reader, cache repoCache) ([]PkgInfo, error) {
	const maxSize = 16 * (1 << 20) // go.mod files are limited to 16 MiB
	data, err := io.ReadAll(io.LimitReader(r, maxSize))
	if err != nil {
		return nil, err
	}

	f, err := modfile.ParseLax("go.mod", data, nil)
	if err != nil {
		return nil, err
	}

	sort.Slice(f.Require, func(i, j int) bool {
		return f.Require[i].Mod.Path < f.Require[j].Mod.Path
	})

	var infos []PkgInfo
	for _, require := range f.Require {
		if require.Indirect {
			continue
		}

		pkg := require.Mod.Path
		pkgName := pkg // for proxy
		if !strings.HasPrefix(pkg, "github.com") {
			if resolved, ok := cache.Get(pkgName); ok {
				pkg = resolved
			} else {
				ctx, cancel := context.WithTimeout(context.Background(), httpTimeout)
				var err error
				pkg, err = proxyRepo(ctx, pkg)
				cancel()
				if err != nil {
					infos = append(infos, PkgInfo{Name: pkgName, Version: require.Mod.Version, Desc: fmt.Sprintf("error: %s", err)})
					continue
				}
				cache.Set(pkgName, pkg)
			}
		}

		owner, repo := repoInfo(pkg)
		if owner == "" || repo == "" {
			slog.Warn("can't get info", "package", pkg)
			continue
		}

		key := fmt.Sprintf("%s/%s", owner, repo)
		desc, ok := cache.Get(key)
		if !ok {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			var err error
			desc, err = repoDesc(ctx, owner, repo)
			cancel()
			if err != nil {
				slog.Error("can't get description", "package", pkgName, "repo", pkg, "error", err)
				continue
			}
			cache.Set(key, desc)
		}

		infos = append(infos, PkgInfo{Name: pkgName, Version: require.Mod.Version, Desc: desc, URL: fmt.Sprintf("https://github.com/%s/%s", owner, repo)})
	}

	return infos, nil
}

var (
	pkgFormat string
)

func init() {
	if isatty.IsTerminal(os.Stdout.Fd()) {
		pkgFormat = "\033[1m%s\033[0m \033[3m%s\033[0m:\n\t%s\n"
	} else {
		pkgFormat = "%s %s:\n\t%s\n"
	}
}

func displayInfo(pkg, version, desc string) {
	fmt.Printf(pkgFormat, pkg, version, desc)
}

func buildVersion() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "dev"
	}

	version := info.Main.Version
	if version == "" || version == "(devel)" {
		return "devel"
	}

	return version
}

func repoDesc(ctx context.Context, owner, repo string) (string, error) {
	url := fmt.Sprintf("%s/repos/%s/%s", githubAPIBase, url.PathEscape(owner), url.PathEscape(repo))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}

	token := os.Getenv(tokenKey)
	if token != "" {
		req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", token))
	}

	resp, err := httpClient.Do(req) //#nosec G704
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("%q: %s", url, resp.Status)
	}

	var reply struct {
		Description string
	}

	if err := json.NewDecoder(resp.Body).Decode(&reply); err != nil {
		return "", fmt.Errorf("%q: can't decode JSON - %w", url, err)
	}

	if reply.Description != "" {
		return reply.Description, nil
	}

	desc, err := readmeDesc(ctx, owner, repo)
	if err != nil {
		slog.Debug("can't get README description", "owner", owner, "repo", repo, "error", err)
		return "", nil
	}
	return desc, nil
}

func readmeDesc(ctx context.Context, owner, repo string) (string, error) {
	rawURL := fmt.Sprintf("%s/%s/%s/HEAD/README.md", githubRawBase,
		url.PathEscape(owner), url.PathEscape(repo))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", err
	}

	token := os.Getenv(tokenKey)
	if token != "" {
		req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", token))
	}

	resp, err := httpClient.Do(req) //#nosec G704
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("%q: %s", rawURL, resp.Status)
	}

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "#") {
			return strings.TrimSpace(strings.TrimLeft(line, "#")), nil
		}
	}

	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("read README: %w", err)
	}
	return "", fmt.Errorf("no header found in README")
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
	fields := strings.Split(strings.TrimPrefix(u.Path, "/"), "/")
	if len(fields) < 5 {
		return "", fmt.Errorf("%q too short", ghURL)
	}

	if fields[2] != "blob" {
		return "", fmt.Errorf("%q: expected /blob/ path", ghURL)
	}

	owner, repo, branch := fields[0], fields[1], fields[3]
	file := strings.Join(fields[4:], "/")
	if file == "" {
		return "", fmt.Errorf("%q: missing file path", ghURL)
	}

	u.Host = "raw.githubusercontent.com"
	path, err := url.JoinPath(owner, repo, branch, file)
	if err != nil {
		return "", fmt.Errorf("can't construct URL - %w", err)
	}

	u.Path = path
	return u.String(), nil
}

func openURL(rawURL string) (io.ReadCloser, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("%q: bad URL- %w", rawURL, err)
	}

	host := strings.ToLower(parsed.Hostname())
	if host == "github.com" {
		var err error
		rawURL, err = githubRawURL(rawURL)
		if err != nil {
			return nil, fmt.Errorf("%q: bad URL- %w", rawURL, err)
		}
		parsed, err = url.Parse(rawURL)
		if err != nil {
			return nil, fmt.Errorf("%q: bad URL- %w", rawURL, err)
		}
	}

	req, err := http.NewRequest(http.MethodGet, rawURL, nil) //#nosec G704
	if err != nil {
		return nil, fmt.Errorf("%q: bad URL- %w", rawURL, err)
	}

	if token := os.Getenv(tokenKey); token != "" {
		host = strings.ToLower(parsed.Hostname())
		if host == "github.com" || host == "raw.githubusercontent.com" {
			req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", token))
		}
	}

	client := &http.Client{Timeout: httpTimeout, Transport: httpClient.Transport}
	resp, err := client.Do(req) //#nosec G704
	if err != nil {
		return nil, fmt.Errorf("%q: can't get- %w", rawURL, err)
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close() //#nosec CWE-703
		return nil, fmt.Errorf("%q: bad status - %s", rawURL, resp.Status)
	}

	return resp.Body, nil
}
