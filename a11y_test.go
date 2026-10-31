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

// Accessibility fixes the built pages keep (#28): every code block can be
// scrolled with the keyboard, the chart links stay reachable for assistive
// technology, the OWASP page has no heading-level jump, and no <pre> carries
// an aria-label.
func TestBuild_Accessibility(t *testing.T) {
	out := buildFixture(t)
	preRe := regexp.MustCompile(`<pre\b[^>]*>`)
	err := filepath.WalkDir(out, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".html") {
			return err
		}
		rel, _ := filepath.Rel(out, path)
		page := read(t, path)
		for _, pre := range preRe.FindAllString(page, -1) {
			if !strings.Contains(pre, `tabindex="0"`) || strings.Contains(pre, "aria-label") {
				t.Errorf("%s: %s, want tabindex=\"0\" and no aria-label", rel, pre)
			}
		}
		if strings.Contains(page, `role="img"`) {
			t.Errorf(`%s: role="img" hides the links inside a chart`, rel)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	owasp := read(t, filepath.Join(out, "owasp", "index.html"))
	if h2, h3 := strings.Index(owasp, `<h2 id="llm">`), strings.Index(owasp, "<h3>"); h2 < 0 || h3 < h2 {
		t.Errorf("/owasp/: the first risk heading comes before its <h2>")
	}
	if !strings.Contains(read(t, filepath.Join(out, "index.html")), `<pre tabindex="0">`) {
		t.Error("home page: no code block found; the check above would pass on nothing")
	}
}
