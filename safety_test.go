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

// A rule doc comes from the cfgaudit release, and the site renders it into
// every rule page. Nothing in it may reach the page as markup or as a link
// that runs script (#27). goldmark's defaults hold this today (raw HTML off,
// dangerous URLs dropped); this pins them, link fixer included. Enabling
// html.WithUnsafe() makes it fail.
func TestConvert_NoScript(t *testing.T) {
	doc := strings.Join([]string{
		"# CFG999: hostile doc",
		"",
		"<script>alert(1)</script>",
		"",
		`<img src="x" onerror="alert(1)">`,
		"",
		`Inline <span onmouseover="alert(1)">html</span> and <a href="javascript:alert(1)">a raw link</a>.`,
		"",
		"[lower](javascript:alert(1)) [upper](JAVASCRIPT:alert(1)) [spaced]( javascript:alert(1))",
		"[slashes](javascript://%0aalert(1)) [vb](vbscript:msgbox(1)) [data](data:text/html,<script>alert(1)</script>)",
		"[entity](javascript&#58;alert(1)) ![img](javascript:alert(1)) <javascript:alert(1)>",
		"",
		"[ref][r]",
		"",
		"[r]: javascript:alert(1)",
		"",
		"| a | b |",
		"|---|---|",
		`| <script>alert(1)</script> | <b onclick="alert(1)">x</b> |`,
	}, "\n")
	html, err := converter{version: "1.18.1"}.convert("docs/rules/CFG999.md", []byte(doc))
	if err != nil {
		t.Fatal(err)
	}
	page := strings.ToLower(string(html))
	for _, bad := range []string{"<script", "<img src=\"x\"", "onerror", "onmouseover", "onclick", "<span", "<b ", `href="javascript:`, `href="vbscript:`, `href="data:`, `src="javascript:`, "<a href=\"javascript"} {
		if strings.Contains(page, bad) {
			t.Errorf("rendered doc contains %q:\n%s", bad, html)
		}
	}
	// A link that still names javascript: may only survive as an inert
	// repository URL from the link fixer, never as a scheme of its own.
	if m := regexp.MustCompile(`href="\s*(javascript|vbscript|data):`).FindString(page); m != "" {
		t.Errorf("script URL in a link: %s", m)
	}
	if !strings.Contains(page, "<!-- raw html omitted -->") {
		t.Errorf("goldmark no longer omits raw HTML:\n%s", html)
	}
}

var (
	cspRe          = regexp.MustCompile(`<meta http-equiv="Content-Security-Policy" content="([^"]*)">`)
	forbiddenTagRe = regexp.MustCompile(`(?i)<(script|iframe|object|embed|form|base)[\s>]`)
	resourceRe     = regexp.MustCompile(`(?i)<(?:img|link|source)\b[^>]*\b(?:src|href)="([^"]*)"`)
)

// Every built page carries the same Content-Security-Policy before its first
// <link>, and nothing that would need a looser one: no script, frame, object,
// form or base element, and no image, stylesheet or icon from another origin
// (#27). A change that needs more fails here first.
func TestBuild_ContentSecurityPolicy(t *testing.T) {
	out := buildFixture(t)
	want := cspRe.FindStringSubmatch(cspMeta)[1]
	pages := 0
	err := filepath.WalkDir(out, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".html") {
			return err
		}
		pages++
		rel, _ := filepath.Rel(out, path)
		page := read(t, path)
		m := cspRe.FindStringSubmatchIndex(page)
		if m == nil {
			t.Errorf("%s: no Content-Security-Policy", rel)
			return nil
		}
		if got := page[m[2]:m[3]]; got != want {
			t.Errorf("%s: policy %q, want %q", rel, got, want)
		}
		if l := strings.Index(page, "<link"); l >= 0 && l < m[0] {
			t.Errorf("%s: a <link> comes before the policy", rel)
		}
		if tag := forbiddenTagRe.FindString(page); tag != "" {
			t.Errorf("%s: contains %s", rel, tag)
		}
		for _, r := range resourceRe.FindAllStringSubmatch(page, -1) {
			if u := r[1]; !strings.HasPrefix(u, "/") || strings.HasPrefix(u, "//") {
				// Links to other sites are fine; only loaded resources count.
				if strings.HasPrefix(r[0], "<link") && !strings.Contains(r[0], `rel="stylesheet"`) && !strings.Contains(r[0], `rel="icon"`) && !strings.Contains(r[0], `rel="apple-touch-icon"`) {
					continue
				}
				t.Errorf("%s: loads %s from another origin", rel, u)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if pages < 5 {
		t.Fatalf("only %d pages built", pages)
	}
}
