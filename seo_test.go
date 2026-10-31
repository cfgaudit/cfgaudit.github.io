// Copyright the cfgaudit contributors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"io/fs"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var (
	titleRe     = regexp.MustCompile(`<title>([^<]*)</title>`)
	descRe      = regexp.MustCompile(`<meta name="description" content="([^"]*)">`)
	canonicalRe = regexp.MustCompile(`<link rel="canonical" href="([^"]*)">`)
	ogURLRe     = regexp.MustCompile(`<meta property="og:url" content="([^"]*)">`)
	h1Re        = regexp.MustCompile(`<h1[\s>]`)
	locRe       = regexp.MustCompile(`<loc>([^<]*)</loc>`)
)

// Every indexable page has one non-empty title, description, canonical URL
// and og:url, the last two its own address, and one h1. No two pages share a
// title or a description, and sitemap.xml lists exactly the indexable pages
// (#29). Redirect pages carry noindex and are left out of both.
func TestBuild_SEO(t *testing.T) {
	out := buildFixture(t)
	titles, descs := map[string]string{}, map[string]string{}
	pages := map[string]bool{}
	err := filepath.WalkDir(out, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".html") {
			return err
		}
		rel, _ := filepath.Rel(out, path)
		page := read(t, path)
		if strings.Contains(page, `<meta name="robots" content="noindex">`) {
			return nil
		}
		head, _, _ := strings.Cut(page, "</head>")
		url := siteURL + "/" + strings.TrimSuffix(filepath.ToSlash(rel), "index.html")
		one := func(re *regexp.Regexp, what string) string {
			m := re.FindAllStringSubmatch(head, -1)
			if len(m) != 1 || strings.TrimSpace(m[0][1]) == "" {
				t.Errorf("%s: %d %s in <head>, want one non-empty", rel, len(m), what)
				return ""
			}
			return m[0][1]
		}
		title, desc := one(titleRe, "<title>"), one(descRe, "meta description")
		for _, c := range []struct {
			got, what string
		}{{one(canonicalRe, "canonical"), "canonical"}, {one(ogURLRe, "og:url"), "og:url"}} {
			if c.got != "" && c.got != url {
				t.Errorf("%s: %s %s, want %s", rel, c.what, c.got, url)
			}
		}
		if n := len(h1Re.FindAllString(page, -1)); n != 1 {
			t.Errorf("%s: %d <h1>, want one", rel, n)
		}
		if other, dup := titles[title]; dup && title != "" {
			t.Errorf("%s and %s share the title %q", rel, other, title)
		}
		if other, dup := descs[desc]; dup && desc != "" {
			t.Errorf("%s and %s share the description %q", rel, other, desc)
		}
		titles[title], descs[desc] = rel, rel
		if rel != "404.html" {
			pages[url] = true
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	listed := map[string]bool{}
	for _, m := range locRe.FindAllStringSubmatch(read(t, filepath.Join(out, "sitemap.xml")), -1) {
		listed[m[1]] = true
		if !pages[m[1]] {
			t.Errorf("sitemap lists %s, which is not an indexable page", m[1])
		}
	}
	for url := range pages {
		if !listed[url] {
			t.Errorf("%s is missing from the sitemap", url)
		}
	}
	if len(pages) < 5 {
		t.Fatalf("only %d indexable pages", len(pages))
	}
}
