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

func conventionalExeURLs(site, repo, version string) []string {
	site = strings.TrimRight(site, "/")
	ver := strings.TrimPrefix(strings.TrimSpace(version), "v")
	if ver == "" {
		return nil
	}
	names := []string{
		"Zapret Manager " + ver + ".exe",
		"ZapretManager-" + ver + ".exe",
	}
	tags := []string{ver, "v" + ver}
	var out []string
	for _, tag := range tags {
		for _, name := range names {
			out = append(out, site+"/"+repo+"/releases/download/"+tag+"/"+url.PathEscape(name))
		}
	}
	return out
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

// LatestManagerRelease returns a newer manager version than current and candidate exe URLs.
// If there is no newer release, version and urls are empty and err is nil.
func (c *Client) LatestManagerRelease(ctx context.Context, current string) (string, []string, error) {
	htmlPage, finalURL, err := c.getHTML(ctx, c.pageURL("/releases/latest"))
	if err != nil {
		return "", nil, err
	}
	tag := tagFromURL(finalURL, c.repo())
	if tag == "" {
		tags := parseTags(htmlPage, c.repo())
		if len(tags) == 0 {
			return "", nil, nil
		}
		tag = tags[0]
	}
	if version.Compare(tag, current) <= 0 {
		return "", nil, nil
	}

	urls := parseExeDownloadURLs(htmlPage, c.site(), c.repo())
	for _, assetTag := range []string{tag, "v" + strings.TrimPrefix(tag, "v")} {
		assetsPage, _, assetsErr := c.getHTML(ctx, c.pageURL("/releases/expanded_assets/"+assetTag))
		if assetsErr == nil {
			urls = append(urls, parseExeDownloadURLs(assetsPage, c.site(), c.repo())...)
		}
	}
	urls = append(urls, conventionalExeURLs(c.site(), c.repo(), tag)...)
	return tag, uniqueURLs(urls), nil
}
