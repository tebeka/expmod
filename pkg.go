/*
	Get repo information from pkg.go.dev.

TODO: Currently we scrape HTML, once they publish an API - switch to it.
*/
package main

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"golang.org/x/net/html"
)

//lint:ignore U1000 WIP
func repoFromPkg(ctx context.Context, dep string) (string, error) {
	url := fmt.Sprintf("https://pkg.go.dev/%s", dep)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("can't get %q - %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("bad status for %q - %s", url, resp.Status)
	}

	return parsePkgHTML(io.LimitReader(resp.Body, 1<<20))
}

/*
Looking for this snippet in the HTML

    <div class="UnitMeta-repo">

        <a href="https://github.com/uber-go/zap" title="https://github.com/uber-go/zap" target="_blank" rel="noopener">
          github.com/uber-go/zap
        </a>

    </div>
*/

func parsePkgHTML(r io.Reader) (string, error) {
	doc, err := html.Parse(r)
	if err != nil {
		return "", err
	}

	pred := func(n *html.Node) bool {
		return n.Data == "div" && attr(n, "class") == "UnitMeta-repo"
	}
	div := findNode(doc, pred)
	if div == nil {
		return "", fmt.Errorf("can't find repo div")
	}

	pred = func(n *html.Node) bool {
		return n.Type == html.ElementNode && n.Data == "a"
	}
	a := findNode(div, pred)
	if a == nil {
		return "", fmt.Errorf("can't find 'a' in repo div")
	}

	return attr(a, "href"), nil
}

func findNode(node *html.Node, pred func(*html.Node) bool) *html.Node {
	if node == nil {
		return nil
	}

	if pred(node) {
		return node
	}

	for n := node.FirstChild; n != nil; n = n.NextSibling {
		if match := findNode(n, pred); match != nil {
			return match
		}
	}

	return nil
}

func attr(n *html.Node, key string) string {
	for _, attr := range n.Attr {
		if attr.Key == key {
			return attr.Val
		}
	}
	return ""
}
