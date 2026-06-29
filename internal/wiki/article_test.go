package wiki_test

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/whither-link/whither/internal/wiki"
)

func TestFetchHTML_OK(t *testing.T) {
	want := []byte("<html><body>stub</body></html>")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write(want)
	}))
	defer srv.Close()

	clients := wiki.NewClients(testConfig(t, srv.URL))
	got, err := clients.Articles.FetchHTML(context.Background(), "Anna's Archive")
	if err != nil {
		t.Fatalf("FetchHTML: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("body mismatch\ngot:  %q\nwant: %q", got, want)
	}
}

func TestFetchHTML_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer srv.Close()

	clients := wiki.NewClients(testConfig(t, srv.URL))
	_, err := clients.Articles.FetchHTML(context.Background(), "NoSuchArticle")
	if !errors.Is(err, wiki.ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestFetchHTML_PathEncoding(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.EscapedPath() // r.URL.Path is always decoded; EscapedPath preserves encoding
		_, _ = w.Write([]byte("<html/>"))
	}))
	defer srv.Close()

	clients := wiki.NewClients(testConfig(t, srv.URL))
	_, _ = clients.Articles.FetchHTML(context.Background(), "Anna's Archive")

	// spaces and apostrophes in the title must be percent-encoded in the path
	if gotPath == "/page/html/Anna's Archive" {
		t.Errorf("title was not URL-encoded in path: %q", gotPath)
	}
}
