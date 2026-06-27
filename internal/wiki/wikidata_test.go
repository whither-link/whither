package wiki_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/whither-link/whither/internal/wiki"
)

func TestOfficialWebsites_Single(t *testing.T) {
	srv := httptest.NewServer(fixtureHandler(t, "wikidata-p856-single.json", "application/json"))
	defer srv.Close()

	// GitHub (Q364): one P856, rank=normal, URL=https://github.com
	clients := wiki.NewClients(testConfig(t, srv.URL))
	sites, err := clients.Wikidata.OfficialWebsites(context.Background(), "Q364")
	if err != nil {
		t.Fatalf("OfficialWebsites: %v", err)
	}
	if len(sites) != 1 {
		t.Fatalf("len(sites) = %d, want 1", len(sites))
	}
	if sites[0].URL != "https://github.com" {
		t.Errorf("URL = %q, want https://github.com", sites[0].URL)
	}
	if sites[0].Rank != "normal" {
		t.Errorf("Rank = %q, want normal", sites[0].Rank)
	}
}

func TestOfficialWebsites_Multi(t *testing.T) {
	srv := httptest.NewServer(fixtureHandler(t, "wikidata-p856-multi.json", "application/json"))
	defer srv.Close()

	// Internet Archive (Q461): two P856: preferred=archive.org, normal=.onion
	clients := wiki.NewClients(testConfig(t, srv.URL))
	sites, err := clients.Wikidata.OfficialWebsites(context.Background(), "Q461")
	if err != nil {
		t.Fatalf("OfficialWebsites: %v", err)
	}
	if len(sites) != 2 {
		t.Fatalf("len(sites) = %d, want 2", len(sites))
	}

	var preferredURL string
	for _, s := range sites {
		if s.Rank == "preferred" {
			preferredURL = s.URL
		}
	}
	if preferredURL != "https://archive.org/" {
		t.Errorf("preferred URL = %q, want https://archive.org/", preferredURL)
	}
}

func TestOfficialWebsites_Empty(t *testing.T) {
	srv := httptest.NewServer(fixtureHandler(t, "wikidata-p856-empty.json", "application/json"))
	defer srv.Close()

	clients := wiki.NewClients(testConfig(t, srv.URL))
	_, err := clients.Wikidata.OfficialWebsites(context.Background(), "Q999")
	if !errors.Is(err, wiki.ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestOfficialWebsites_NoValue(t *testing.T) {
	srv := httptest.NewServer(fixtureHandler(t, "wikidata-p856-novalue.json", "application/json"))
	defer srv.Close()

	clients := wiki.NewClients(testConfig(t, srv.URL))
	_, err := clients.Wikidata.OfficialWebsites(context.Background(), "Q999")
	if !errors.Is(err, wiki.ErrNotFound) {
		t.Errorf("novalue snak should yield ErrNotFound, got %v", err)
	}
}

func TestOfficialWebsites_BadJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("{bad json"))
	}))
	defer srv.Close()

	clients := wiki.NewClients(testConfig(t, srv.URL))
	_, err := clients.Wikidata.OfficialWebsites(context.Background(), "Q1")
	if !errors.Is(err, wiki.ErrBadResponse) {
		t.Errorf("err = %v, want ErrBadResponse", err)
	}
}
