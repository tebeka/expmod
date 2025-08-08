package main

import (
	"os"
	"testing"
)

var htmlCases = []struct {
	file string
	repo string
}{
	{"testdata/yaml.html", "github.com/go-yaml/yaml"},
	{"testdata/zap.html", "github.com/uber-go/zap"},
	{"testdata/k8s.html", "github.com/kubernetes/kubernetes"},
}

func Test_parseProxyHTML(t *testing.T) {
	for _, tc := range htmlCases {
		t.Run(tc.file, func(t *testing.T) {
			file, err := os.Open(tc.file)
			if err != nil {
				t.Fatalf("open: %v", err)
			}
			defer file.Close()

			repo, err := parseProxyHTML(file)
			if err != nil {
				t.Fatalf("parse HTML: %v", err)
			}
			if repo != tc.repo {
				t.Fatalf("expected %q, got %q", tc.repo, repo)
			}
		})
	}
}

func Test_proxyRepo(t *testing.T) {
	if os.Getenv("CI") != "" {
		t.Skip("in CI")
	}

	pkg := "gopkg.in/yaml.v3"
	ctx, cancel := testCtx(t)
	defer cancel()
	repo, err := proxyRepo(ctx, pkg)
	if err != nil {
		t.Fatalf("proxyRepo: %v", err)
	}
	expected := "github.com/go-yaml/yaml"
	if repo != expected {
		t.Fatalf("expected %q, got %q", expected, repo)
	}
}
