// HTML utilities
package main

import "golang.org/x/net/html"

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
