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
func (c *lruCache) Set(key, value string)          { c.c.Add(key, value) }

type server struct{ cache *lruCache }

func newServer(cacheSize int) (*server, error) {
	c, err := lru.New[string, string](cacheSize)
	if err != nil {
		return nil, err
	}
	return &server{cache: &lruCache{c: c}}, nil
}

func (s *server) pkgsFromRequest(r *http.Request) ([]PkgInfo, error) {
	repo := r.FormValue("repo")
	var rc io.ReadCloser
	if repo != "" {
		uri := fmt.Sprintf("https://github.com/%s/blob/main/go.mod", repo)
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
	pkgs, err := s.pkgsFromRequest(r)
	if err != nil {
		fmt.Fprintf(w, `<p class="error">%s</p>`, template.HTMLEscapeString(err.Error()))
		return
	}
	if err := resultsTmpl.Execute(w, pkgs); err != nil {
		slog.Error("render results", "error", err)
	}
}

func (s *server) handleAPI(w http.ResponseWriter, r *http.Request) {
	pkgs, err := s.pkgsFromRequest(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(pkgs)
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
