package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	lru "github.com/hashicorp/golang-lru/v2"
)

//go:embed templates
var templatesFS embed.FS

var (
	pageTmpl    = template.Must(template.ParseFS(templatesFS, "templates/page.html"))
	resultsTmpl = template.Must(template.ParseFS(templatesFS, "templates/results.html"))
)

type lruCache struct{ c *lru.Cache[string, string] }

func (c *lruCache) Get(key string) (string, bool) { return c.c.Get(key) }
func (c *lruCache) Set(key, value string)         { c.c.Add(key, value) }

type server struct{ cache *lruCache }

const maxFormBytes = 2 << 20

var githubRawBase = "https://raw.githubusercontent.com"

func newServer(cacheSize int) (*server, error) {
	c, err := lru.New[string, string](cacheSize)
	if err != nil {
		return nil, err
	}
	return &server{cache: &lruCache{c: c}}, nil
}

func (s *server) pkgsFromRequest(w http.ResponseWriter, r *http.Request) ([]PkgInfo, error) {
	if r.Method == http.MethodPost {
		r.Body = http.MaxBytesReader(w, r.Body, maxFormBytes)
		if err := r.ParseForm(); err != nil {
			return nil, err
		}
	}

	repo := r.FormValue("repo")
	var rc io.ReadCloser
	if repo != "" {
		uri := fmt.Sprintf("%s/%s/HEAD/go.mod", githubRawBase, repo)
		var err error
		rc, err = openURL(uri)
		if err != nil {
			return nil, err
		}
		defer rc.Close()
	} else {
		rc = io.NopCloser(strings.NewReader(r.FormValue("content")))
	}
	return pkgsInfo(rc, s.cache)
}

func (s *server) handlePage(w http.ResponseWriter, r *http.Request) {
	if err := pageTmpl.Execute(w, nil); err != nil {
		slog.Error("render page", "error", err)
	}
}

func (s *server) handleHTMX(w http.ResponseWriter, r *http.Request) {
	pkgs, err := s.pkgsFromRequest(w, r)
	if err != nil {
		fmt.Fprint(w, `<p class="error">`)
		template.HTMLEscape(w, []byte(err.Error()))
		fmt.Fprint(w, `</p>`)
		return
	}
	if err := resultsTmpl.Execute(w, pkgs); err != nil {
		slog.Error("render results", "error", err)
	}
}

func (s *server) handleAPI(w http.ResponseWriter, r *http.Request) {
	pkgs, err := s.pkgsFromRequest(w, r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(pkgs); err != nil {
		slog.Error("encode response", "error", err)
	}
}

func serve(addr string) {
	s, err := newServer(512)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %s\n", err)
		os.Exit(1)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /", s.handlePage)
	mux.HandleFunc("POST /", s.handleHTMX)
	mux.HandleFunc("POST /api", s.handleAPI)

	srv := &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 2 * time.Minute,
		IdleTimeout:  2 * time.Minute,
	}

	slog.Info("listening", "addr", addr)
	if err := srv.ListenAndServe(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %s\n", err)
		os.Exit(1)
	}
}
