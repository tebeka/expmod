package main

import (
	"io"
	"net/http"
	"os"
	"strings"
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
	oldClient := httpClient
	httpClient = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.String() != "https://gopkg.in/yaml.v3?go-get=1" {
			t.Fatalf("unexpected URL %q", req.URL.String())
		}

		file, err := os.Open("testdata/yaml.html")
		if err != nil {
			t.Fatalf("open fixture: %v", err)
		}

		t.Cleanup(func() {
			file.Close()
		})

		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       file,
			Header:     make(http.Header),
		}, nil
	})}
	t.Cleanup(func() {
		httpClient = oldClient
	})

	ctx, cancel := testCtx(t)
	defer cancel()
	repo, err := proxyRepo(ctx, "gopkg.in/yaml.v3")
	if err != nil {
		t.Fatalf("proxyRepo: %v", err)
	}

	expected := "github.com/go-yaml/yaml"
	if repo != expected {
		t.Fatalf("expected %q, got %q", expected, repo)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func Test_proxyRepoStatusError(t *testing.T) {
	oldClient := httpClient
	httpClient = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			Status:     "502 Bad Gateway",
			StatusCode: http.StatusBadGateway,
			Body:       io.NopCloser(strings.NewReader("bad gateway")),
			Header:     make(http.Header),
		}, nil
	})}
	t.Cleanup(func() {
		httpClient = oldClient
	})

	ctx, cancel := testCtx(t)
	defer cancel()

	_, err := proxyRepo(ctx, "gopkg.in/yaml.v3")
	if err == nil {
		t.Fatal("expected error")
	}
}
