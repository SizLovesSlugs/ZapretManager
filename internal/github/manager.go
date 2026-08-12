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

	urls := conventionalExeURLs(c.site(), c.repo(), tag)
	urls = append(urls, parseExeDownloadURLs(htmlPage, c.site(), c.repo())...)
	for _, assetTag := range uniqueURLs([]string{tag, "v" + strings.TrimPrefix(tag, "v"), strings.ReplaceAll(tag, " ", "-")}) {
		assetsPage, _, assetsErr := c.getHTML(ctx, c.pageURL("/releases/expanded_assets/"+assetTag))
		if assetsErr == nil {
			urls = append(urls, parseExeDownloadURLs(assetsPage, c.site(), c.repo())...)
		}
	}
	return tag, uniqueURLs(urls), nil
}
