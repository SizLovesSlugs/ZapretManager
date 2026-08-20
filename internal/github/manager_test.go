package github

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestParseExeDownloadURLs(t *testing.T) {
	page := `
		<a href="/SizLovesSlugs/ZapretManager/releases/download/0.2/Zapret%20Manager%200.2.exe">exe</a>
		<a href="/SizLovesSlugs/ZapretManager/releases/download/0.2/notes.txt">txt</a>
	`
	got := parseExeDownloadURLs(page, "https://github.com", ManagerRepo)
	if len(got) != 1 || !strings.Contains(got[0], "Zapret%20Manager%200.2.exe") {
		t.Fatalf("%v", got)
	}
}

func TestConventionalExeURLs(t *testing.T) {
	got := conventionalExeURLs("https://github.com", ManagerRepo, "1.0")
	want := "https://github.com/SizLovesSlugs/ZapretManager/releases/download/1.0/ZapretManager-1.0.exe"
	if len(got) < 1 || got[0] != want {
		t.Fatalf("%v", got)
	}
}

func TestLatestManagerAsset(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/SizLovesSlugs/ZapretManager/releases/latest", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(apiRelease{
			TagName: "1.1.7",
			Assets: []apiAsset{{
				Name:               "ZapretManager-1.1.7.exe",
				BrowserDownloadURL: "https://github.com/SizLovesSlugs/ZapretManager/releases/download/1.1.7/ZapretManager-1.1.7.exe",
				Digest:             "sha256:d7bed34756e818b2bf9ab60efd2b9ef4888214bc308e190e59ab67f21f31f0ed",
			}},
		})
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	c := NewManager()
	c.HTTP = srv.Client()
	c.APIURL = srv.URL

	got, err := c.LatestManagerAsset(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got.Tag != "1.1.7" || got.Name != "ZapretManager-1.1.7.exe" {
		t.Fatalf("%+v", got)
	}
	if !SameDigest(got.Digest, "D7BED34756E818B2BF9AB60EFD2B9EF4888214BC308E190E59AB67F21F31F0ED") {
		t.Fatalf("digest %q", got.Digest)
	}
}
