package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

const testGoMod = `module example.com/test

go 1.21

require (
	github.com/banana/b v1.0.0
	github.com/apple/a v1.2.3
)
`

func TestHandleAPIRepoInput(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/owner/repo/HEAD/go.mod" {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(testGoMod))
	}))
	defer ts.Close()

	prevBase := githubRawBase
	githubRawBase = ts.URL
	defer func() { githubRawBase = prevBase }()

	srv, err := newServer(8)
	if err != nil {
		t.Fatalf("newServer: %v", err)
	}
	srv.cache.Set("apple/a", "desc A")
	srv.cache.Set("banana/b", "desc B")

	form := url.Values{}
	form.Set("repo", "owner/repo")
	req := httptest.NewRequest(http.MethodPost, "/api", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	srv.handleAPI(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: %d", resp.StatusCode)
	}

	var pkgs []PkgInfo
	if err := json.NewDecoder(resp.Body).Decode(&pkgs); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(pkgs) != 2 {
		t.Fatalf("expected 2 pkgs, got %d", len(pkgs))
	}
	if pkgs[0].Name != "github.com/apple/a" || pkgs[1].Name != "github.com/banana/b" {
		t.Fatalf("unexpected order: %v", []string{pkgs[0].Name, pkgs[1].Name})
	}
}

func TestHandleAPIContentInput(t *testing.T) {
	srv, err := newServer(8)
	if err != nil {
		t.Fatalf("newServer: %v", err)
	}
	srv.cache.Set("apple/a", "desc A")
	srv.cache.Set("banana/b", "desc B")

	form := url.Values{}
	form.Set("content", testGoMod)
	req := httptest.NewRequest(http.MethodPost, "/api", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	srv.handleAPI(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: %d", resp.StatusCode)
	}

	var pkgs []PkgInfo
	if err := json.NewDecoder(resp.Body).Decode(&pkgs); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(pkgs) != 2 {
		t.Fatalf("expected 2 pkgs, got %d", len(pkgs))
	}
	if pkgs[0].Name != "github.com/apple/a" || pkgs[1].Name != "github.com/banana/b" {
		t.Fatalf("unexpected order: %v", []string{pkgs[0].Name, pkgs[1].Name})
	}
}

func TestHandleHTMX(t *testing.T) {
	srv, err := newServer(8)
	if err != nil {
		t.Fatalf("newServer: %v", err)
	}
	srv.cache.Set("apple/a", "desc A")

	form := url.Values{}
	form.Set("content", `module example.com/test

go 1.21

require github.com/apple/a v1.2.3
`)
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	srv.handleHTMX(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: %d", resp.StatusCode)
	}
	body := w.Body.String()
	if !strings.Contains(body, "github.com/apple/a") {
		t.Fatalf("expected package name in response")
	}
}

func TestHandlePage(t *testing.T) {
	srv, err := newServer(8)
	if err != nil {
		t.Fatalf("newServer: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	srv.handlePage(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: %d", resp.StatusCode)
	}
	if w.Body.Len() == 0 {
		t.Fatalf("expected body")
	}
}

func TestHandleAPITooLarge(t *testing.T) {
	srv, err := newServer(8)
	if err != nil {
		t.Fatalf("newServer: %v", err)
	}

	payload := "content=" + strings.Repeat("a", maxFormBytes+10)
	req := httptest.NewRequest(http.MethodPost, "/api", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	srv.handleAPI(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status: %d", resp.StatusCode)
	}
}

func TestHandleAPIBothInputsSet(t *testing.T) {
	srv, err := newServer(8)
	if err != nil {
		t.Fatalf("newServer: %v", err)
	}

	form := url.Values{}
	form.Set("repo", "owner/repo")
	form.Set("content", testGoMod)
	req := httptest.NewRequest(http.MethodPost, "/api", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	srv.handleAPI(w, req)

	if w.Result().StatusCode != http.StatusBadRequest {
		t.Fatalf("status: %d", w.Result().StatusCode)
	}
}

func TestHandleAPIMissingInput(t *testing.T) {
	srv, err := newServer(8)
	if err != nil {
		t.Fatalf("newServer: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api", strings.NewReader(""))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	srv.handleAPI(w, req)

	if w.Result().StatusCode != http.StatusBadRequest {
		t.Fatalf("status: %d", w.Result().StatusCode)
	}
}
