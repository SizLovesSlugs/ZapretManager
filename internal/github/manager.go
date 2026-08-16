package github

import (
	"context"
	"html"
	"net/url"
	"regexp"
	"strings"

	"zapret-manager/internal/version"
)

const ManagerRepo = "SizLovesSlugs/ZapretManager"

func NewManager() *Client {
	c := New()
	c.Repo = ManagerRepo
	return c
}

type ManagerAsset struct {
	Tag    string
	Name   string
	URL    string
	Digest string
}

type apiRelease struct {
	TagName string     `json:"tag_name"`
	Assets  []apiAsset `json:"assets"`
}

type apiAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Digest             string `json:"digest"`
}

func pickManagerExe(assets []apiAsset) (apiAsset, bool) {
	var fallback apiAsset
	var hasFallback bool
	for _, a := range assets {
		name := strings.ToLower(strings.TrimSpace(a.Name))
		if !strings.HasSuffix(name, ".exe") || a.BrowserDownloadURL == "" {
			continue
		}
		if strings.Contains(name, "zapret") && strings.Contains(name, "manager") {
			return a, true
		}
		if !hasFallback {
			fallback = a
			hasFallback = true
		}
	}
	return fallback, hasFallback
}

// LatestManagerAsset is the exe from GitHub /releases/latest, including its sha256 digest.
func (c *Client) LatestManagerAsset(ctx context.Context) (ManagerAsset, error) {
	var rel apiRelease
	if err := c.getJSON(ctx, c.apiRoot()+"/repos/"+c.repo()+"/releases/latest", &rel); err != nil {
		return ManagerAsset{}, err
	}
	a, ok := pickManagerExe(rel.Assets)
	if !ok {
		return ManagerAsset{}, nil
	}
	return ManagerAsset{
		Tag:    strings.TrimPrefix(strings.TrimSpace(rel.TagName), "v"),
		Name:   a.Name,
		URL:    a.BrowserDownloadURL,
		Digest: NormalizeDigest(a.Digest),
	}, nil
}

var exeAssetRe = regexp.MustCompile(`(?i)/releases/download/([^"'?#\s]+)/([^"'?#\s]+\.exe)`)

func parseExeDownloadURLs(page, site, repo string) []string {
	site = strings.TrimRight(site, "/")
	seen := map[string]bool{}
	var preferred, other []string
	for _, m := range exeAssetRe.FindAllStringSubmatch(page, -1) {
		tag := strings.TrimSpace(html.UnescapeString(m[1]))
		file := strings.TrimSpace(html.UnescapeString(m[2]))
		if tag == "" || file == "" {
			continue
		}
		raw := site + "/" + repo + "/releases/download/" + tag + "/" + file
		if seen[raw] {
			continue
		}
		seen[raw] = true
		name, err := url.PathUnescape(file)
		if err != nil {
			name = file
		}
		lower := strings.ToLower(name)
		if strings.Contains(lower, "zapret") && strings.Contains(lower, "manager") {
			preferred = append(preferred, raw)
			continue
		}
		other = append(other, raw)
	}
	return append(preferred, other...)
}

func conventionalExeURLs(site, repo, tag string) []string {
	site = strings.TrimRight(site, "/")
	ver := strings.TrimPrefix(strings.TrimSpace(tag), "v")
	if ver == "" {
		return nil
	}
	name := version.ExeNameFor(ver)
	tags := []string{ver, "v" + ver}
	if dashed := strings.ReplaceAll(ver, " ", "-"); dashed != ver {
		tags = append(tags, dashed, "v"+dashed)
	}
	var out []string
	for _, t := range tags {
		out = append(out, site+"/"+repo+"/releases/download/"+t+"/"+url.PathEscape(name))
	}
	return uniqueURLs(out)
}

func uniqueURLs(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, u := range in {
		if u == "" || seen[u] {
			continue
		}
		seen[u] = true
		out = append(out, u)
	}
	return out
}
