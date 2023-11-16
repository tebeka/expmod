package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"
	"time"
)

func main() {
	flag.Usage = func() {
		name := path.Base(os.Args[0])
		fmt.Fprintf(os.Stderr, "usage: %s [file]\n", name)
		flag.PrintDefaults()
	}
	flag.Parse()

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

	if err := pkgsInfo(r); err != nil {
		log.Fatalf("error: %s", err)
	}
}

func pkgsInfo(r io.Reader) error {
	s := bufio.NewScanner(r)
	for s.Scan() {
		line := strings.TrimSpace(s.Text())
		if ignored(line) {
			continue
		}

		owner, repo := repoInfo(line)
		if owner == "" || repo == "" {
			fmt.Fprintf(os.Stderr, "error: %s\n", line)
			continue
		}

		desc, err := repoDesc(owner, repo)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %s - %s\n", line, err)
			continue
		}

		fmt.Printf("%s:\n\t%s\n", line, desc)
	}

	if err := s.Err(); err != nil {
		return err
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

func repoInfo(line string) (string, string) {
	// "github.com/go-redis/redis/v8 v8.11.5 // indirect" -> "go-redis", "redis"
	fields := strings.Split(line, "/")
	if len(fields) < 3 {
		return "", ""
	}
	owner := fields[1]
	repo, _, _ := strings.Cut(fields[2], " ")
	return owner, repo
}

func ignored(line string) bool {
	if !strings.HasPrefix(line, "github.com") {
		return true
	}

	if strings.Contains(line, "// indirect") {
		return true
	}

	return false
}
