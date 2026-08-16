package github

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const (
	DefaultRepo = "Flowseal/zapret-discord-youtube"
	userAgent   = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"
)

type Client struct {
	HTTP    *http.Client
	Repo    string
	BaseURL string
	APIURL  string
}

func New() *Client {
	return &Client{
		HTTP:    newHTTPClient(20 * time.Second),
		Repo:    DefaultRepo,
		BaseURL: "https://github.com",
	}
}

func newHTTPClient(timeout time.Duration) *http.Client {
	dialer := &net.Dialer{Timeout: 8 * time.Second, KeepAlive: 30 * time.Second}
	transport := &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           dialer.DialContext,
		ForceAttemptHTTP2:     false,
		TLSNextProto:          map[string]func(string, *tls.Conn) http.RoundTripper{},
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 20 * time.Second,
		IdleConnTimeout:       30 * time.Second,
		MaxIdleConns:          8,
		MaxIdleConnsPerHost:   4,
		DisableKeepAlives:     false,
	}
	return &http.Client{
		Timeout:   timeout,
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 8 {
				return fmt.Errorf("too many redirects")
			}
			return nil
		},
	}
}

type Release struct {
	TagName     string    `json:"tag_name"`
	PublishedAt time.Time `json:"published_at"`
}

func (r Release) Version() string {
	return strings.TrimPrefix(strings.TrimSpace(r.TagName), "v")
}

func ZipName(version string) string {
	return "zapret-discord-youtube-" + strings.TrimPrefix(strings.TrimSpace(version), "v") + ".zip"
}

func ZipDownloadURL(version string) string {
	v := strings.TrimPrefix(strings.TrimSpace(version), "v")
	return "https://github.com/" + DefaultRepo + "/releases/download/" + v + "/" + ZipName(v)
}

func (c *Client) repo() string {
	if c.Repo != "" {
		return c.Repo
	}
	return DefaultRepo
}

func (c *Client) site() string {
	if c.BaseURL != "" {
		return strings.TrimRight(c.BaseURL, "/")
	}
	return "https://github.com"
}

func (c *Client) pageURL(path string) string {
	return c.site() + "/" + c.repo() + path
}

func (c *Client) apiRoot() string {
	if c.APIURL != "" {
		return strings.TrimRight(c.APIURL, "/")
	}
	return "https://api.github.com"
}

func (c *Client) getJSON(ctx context.Context, rawURL string, dest any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("github api %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	return json.NewDecoder(io.LimitReader(resp.Body, 2<<20)).Decode(dest)
}

func browserHeaders(req *http.Request) {
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "ru-RU,ru;q=0.9,en;q=0.8")
	req.Header.Set("Cache-Control", "no-cache")
}

func (c *Client) getHTML(ctx context.Context, url string) (string, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", "", err
	}
	browserHeaders(req)
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return "", "", fmt.Errorf("github %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return "", "", err
	}
	final := url
	if resp.Request != nil && resp.Request.URL != nil {
		final = resp.Request.URL.String()
	}
	return string(raw), final, nil
}

func (c *Client) LatestVersion(ctx context.Context) (string, error) {
	htmlPage, finalURL, err := c.getHTML(ctx, c.pageURL("/releases/latest"))
	if err != nil {
		return "", err
	}
	if tag := tagFromURL(finalURL, c.repo()); tag != "" {
		return tag, nil
	}
	tags := parseTags(htmlPage, c.repo())
	if len(tags) == 0 {
		return "", fmt.Errorf("не удалось прочитать последнюю версию со страницы релизов")
	}
	return tags[0], nil
}

func (c *Client) ListReleases(ctx context.Context) ([]Release, error) {
	htmlPage, _, err := c.getHTML(ctx, c.pageURL("/releases"))
	if err != nil {
		return nil, err
	}
	tags := parseTags(htmlPage, c.repo())
	if len(tags) == 0 {
		return nil, fmt.Errorf("на странице релизов не найдены версии")
	}
	out := make([]Release, 0, len(tags))
	for _, tag := range tags {
		out = append(out, Release{TagName: tag})
	}
	return out, nil
}

func FindRelease(list []Release, version string) (Release, bool) {
	version = strings.TrimPrefix(strings.TrimSpace(version), "v")
	for _, rel := range list {
		if rel.Version() == version {
			return rel, true
		}
	}
	return Release{}, false
}

var (
	tagPathRe    = regexp.MustCompile(`/releases/tag/([^"'?#\s]+)`)
	defaultTagRe = regexp.MustCompile(`/Flowseal/zapret-discord-youtube/releases/tag/([^"'?#\s]+)`)
)

func tagFromURL(raw, repo string) string {
	m := tagPathRe.FindStringSubmatch(raw)
	if len(m) < 2 {
		return ""
	}
	return cleanTag(m[1])
}

func parseTags(page, repo string) []string {
	re := defaultTagRe
	if repo != DefaultRepo {
		re = regexp.MustCompile(regexp.QuoteMeta("/"+repo+"/releases/tag/") + `([^"'?#\s]+)`)
	}
	seen := map[string]bool{}
	var out []string
	for _, m := range re.FindAllStringSubmatch(page, -1) {
		tag := cleanTag(m[1])
		if tag == "" || seen[tag] {
			continue
		}
		seen[tag] = true
		out = append(out, tag)
	}
	return out
}

func cleanTag(tag string) string {
	tag = html.UnescapeString(tag)
	tag = strings.TrimSpace(tag)
	tag = strings.TrimPrefix(tag, "v")
	return tag
}

type ProgressFunc func(received, total int64)

func (c *Client) Download(ctx context.Context, url, dest string, progress ProgressFunc) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Accept-Language", "ru-RU,ru;q=0.9,en;q=0.8")
	req.Header.Set("Referer", c.site()+"/"+c.repo()+"/releases")

	client := *c.HTTP
	client.Timeout = 0
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return fmt.Errorf("download: %s %s", resp.Status, strings.TrimSpace(string(body)))
	}

	tmp := dest + ".part"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	defer func() {
		f.Close()
		os.Remove(tmp)
	}()

	total := resp.ContentLength
	var received int64
	buf := make([]byte, 32*1024)
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			if _, err := f.Write(buf[:n]); err != nil {
				return err
			}
			received += int64(n)
			if progress != nil {
				progress(received, total)
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return readErr
		}
	}
	if err := f.Close(); err != nil {
		return err
	}
	os.Remove(dest)
	return os.Rename(tmp, dest)
}

func NormalizeDigest(digest string) string {
	digest = strings.TrimSpace(strings.ToLower(digest))
	if digest == "" {
		return ""
	}
	if hex, ok := strings.CutPrefix(digest, "sha256:"); ok {
		return "sha256:" + strings.TrimSpace(hex)
	}
	if len(digest) == 64 {
		return "sha256:" + digest
	}
	return digest
}

func SameDigest(a, b string) bool {
	a, b = NormalizeDigest(a), NormalizeDigest(b)
	return a != "" && a == b
}

func FileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil)), nil
}

func VerifySHA256(path, digest string) error {
	digest = NormalizeDigest(digest)
	if digest == "" {
		return nil
	}
	got, err := FileSHA256(path)
	if err != nil {
		return err
	}
	if !SameDigest(got, digest) {
		return fmt.Errorf("checksum mismatch: got %s, want %s", got, digest)
	}
	return nil
}
