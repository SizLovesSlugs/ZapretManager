package github

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestZipDownloadURL(t *testing.T) {
	got := ZipDownloadURL("1.9.9d")
	want := "https://github.com/Flowseal/zapret-discord-youtube/releases/download/1.9.9d/zapret-discord-youtube-1.9.9d.zip"
	if got != want {
		t.Fatal(got)
	}
}

func TestParseTagsFromReleasesHTML(t *testing.T) {
	page := `
		<a href="/Flowseal/zapret-discord-youtube/releases/tag/1.10.1">1.10.1</a>
		<a href="/Flowseal/zapret-discord-youtube/releases/tag/1.10.1">1.10.1</a>
		<a href="/Flowseal/zapret-discord-youtube/releases/tag/1.10.0">1.10.0</a>
		<a href="/Flowseal/zapret-discord-youtube/releases/tag/1.9.9d">1.9.9d</a>
	`
	got := parseTags(page, DefaultRepo)
	if len(got) != 3 || got[0] != "1.10.1" || got[2] != "1.9.9d" {
		t.Fatalf("%v", got)
	}
}

func TestLatestAndListFromPages(t *testing.T) {
	payload := []byte("zapret-pack")
	sum := sha256.Sum256(payload)
	digest := "sha256:" + hex.EncodeToString(sum[:])
	mux := http.NewServeMux()
	mux.HandleFunc("/Flowseal/zapret-discord-youtube/releases/latest", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/Flowseal/zapret-discord-youtube/releases/tag/1.10.1", http.StatusFound)
	})
	mux.HandleFunc("/Flowseal/zapret-discord-youtube/releases/tag/1.10.1", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<a href="/Flowseal/zapret-discord-youtube/releases/tag/1.10.1">1.10.1</a>`))
	})
	mux.HandleFunc("/Flowseal/zapret-discord-youtube/releases", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`
			<a href="/Flowseal/zapret-discord-youtube/releases/tag/1.10.1">1.10.1</a>
			<a href="/Flowseal/zapret-discord-youtube/releases/tag/1.10.0">1.10.0</a>
		`))
	})
	mux.HandleFunc("/file.zip", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(payload)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	c := New()
	c.HTTP = srv.Client()
	c.BaseURL = srv.URL

	ver, err := c.LatestVersion(context.Background())
	if err != nil || ver != "1.10.1" {
		t.Fatalf("latest %q %v", ver, err)
	}
	list, err := c.ListReleases(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 || list[1].Version() != "1.10.0" {
		t.Fatalf("list %+v", list)
	}
	if _, ok := FindRelease(list, "1.10.0"); !ok {
		t.Fatal("FindRelease")
	}

	dest := filepath.Join(t.TempDir(), "pack.zip")
	if err := c.Download(context.Background(), srv.URL+"/file.zip", dest, nil); err != nil {
		t.Fatal(err)
	}
	if err := VerifySHA256(dest, digest); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(dest)
	if string(got) != string(payload) {
		t.Fatalf("content %q", got)
	}
}

func TestSameDigest(t *testing.T) {
	sum := "sha256:d7bed34756e818b2bf9ab60efd2b9ef4888214bc308e190e59ab67f21f31f0ed"
	if !SameDigest(sum, "D7BED34756E818B2BF9AB60EFD2B9EF4888214BC308E190E59AB67F21F31F0ED") {
		t.Fatal("bare hex should match")
	}
	if SameDigest(sum, "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa") {
		t.Fatal("different digest")
	}
}
