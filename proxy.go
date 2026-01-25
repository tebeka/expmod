package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"

	"golang.org/x/net/html"
)

func proxyRepo(ctx context.Context, dep string) (string, error) {
	url := fmt.Sprintf("https://%s?go-get=1", dep)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("GET %q - %w", url, err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GET %q - %s", url, resp.Status)
	}
	defer resp.Body.Close()

	repo, err := parseProxyHTML(resp.Body)
	if err != nil {
		return "", fmt.Errorf("%w in %q", err, url)
	}

	return repo, nil
}

/*
Looking for go-source in HTML
<html>
<head>
<meta name="go-import" content="gopkg.in/yaml.v3 git https://gopkg.in/yaml.v3">
<meta name="go-source" content="gopkg.in/yaml.v3 _ https://github.com/go-yaml/yaml/tree/v3.0.1{/dir} https://github.com/go-yaml/yaml/blob/v3.0.1{/dir}/{file}#L{line}">
</head>
<body>
go get gopkg.in/yaml.v3
</body>
</html>
*/

// gopkg.in/yaml.v3 _ https://github.com/go-yaml/yaml/tree/v3.0.1{/dir} https://github.com/go-yaml/yaml/blob/v3.0.1{/dir}/{file}#L{line} -> https://github.com/go-yaml/yaml
// golang.org/x/term -> go.googlesource.com/term -> https://github.com/golang/go
var ghRE = regexp.MustCompile(`https://(github\.com/[^\s/{}]+/[^\s/{}]+)`)  // Use github\.com to match literal dot, [^\s/{}]+ to stop at whitespace, /, {, or }
var goGooglesourceRE = regexp.MustCompile(`https://go\.googlesource\.com/(\w+)`)

// parseProxyHTML finds github repo in proxy HTML.
func parseProxyHTML(r io.Reader) (string, error) {
	doc, err := html.Parse(r)
	if err != nil {
		return "", err
	}

	// Check go-source first
	names := []string{"go-source", "go-import"}
	for _, name := range names {

		pred := func(n *html.Node) bool { return n.Data == "meta" && attr(n, "name") == name }
		node := findNode(doc, pred)
		if node == nil {
			continue
		}

		src := attr(node, "content")
		if src == "" {
			return "", fmt.Errorf("can't find content in meta")
		}

		// Try GitHub first
		matches := ghRE.FindStringSubmatch(src)
		if len(matches) > 0 {
			return strings.TrimSpace(matches[1]), nil
		}

		// Handle golang.org/x modules (go.googlesource.com/{name} -> github.com/golang/{name})
		if matches := goGooglesourceRE.FindStringSubmatch(src); len(matches) > 0 {
			return fmt.Sprintf("github.com/golang/%s", matches[1]), nil
		}

		return "", fmt.Errorf("can't find github repo in meta")
	}

	return "", fmt.Errorf("non of %s found in metadata", strings.Join(names, ", "))
}
