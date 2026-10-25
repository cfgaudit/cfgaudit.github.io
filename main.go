// Copyright the cfgaudit contributors
// SPDX-License-Identifier: Apache-2.0

// Command site builds cfgaudit.github.io from a cfgaudit source tree: a home
// page, the rule overview, one page per docs/rules/CFGxxx.md, the OWASP page
// and the example reports.
//
//	go run . -src ../cfgaudit -version 1.18.1 -reports reports -rules rules.json -out _site
package main

import (
	"bytes"
	"embed"
	"errors"
	"flag"
	"fmt"
	"html/template"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

const (
	siteURL = "https://cfgaudit.github.io"
	repoURL = "https://github.com/cfgaudit/cfgaudit"
)

//go:embed templates/*.html
var templateFS embed.FS

//go:embed static
var staticFS embed.FS

var (
	ruleFileRe = regexp.MustCompile(`^docs/rules/(CFG[0-9]{3})\.md$`)
	versionRe  = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+$`)
	// The first line of a rule doc: "# CFG001 — `permissions.allow` contains
	// ..." or, in a few docs, "# CFG110: a Claude Code settings env block ...".
	ruleTitleRe = regexp.MustCompile(`^# (CFG[0-9]{3})(?: [\x{2014}\x{2013}-]|:) (.+)$`)
)

// Rule is one rule page.
type Rule struct {
	ID        string
	Title     string // plain text, for <title>
	TitleHTML template.HTML
	Body      template.HTML
	Listed    ListedRule
}

// Page is what every template gets.
type Page struct {
	Title       string
	Description string
	Path        string // site path, "/" or "/rules/CFG001/"
	Version     string
	Sample      []string // lines of the home page example
	Reports     []Report
	Inputs      []SampleFile // the files the reports are for
	Risks       []Risk       // OWASP LLM Top 10
	MCPRisks    []Risk       // OWASP MCP Top 10
	Covered     int          // LLM risks with at least one rule
	Outside     int          // LLM risks a configuration file can't show
	MCPCovered  int
	Agents      []Agent // agent sections of the README at the release tag
	Rule        *Rule
	RuleCount   int
}

func main() {
	src := flag.String("src", "", "cfgaudit source tree, checked out at a release tag")
	version := flag.String("version", "", "cfgaudit release the source tree is at, e.g. 1.18.1")
	reports := flag.String("reports", "", "directory with cfgaudit's output on the sample, one <format>.txt per format, and the sample under input/")
	rules := flag.String("rules", "", "output of cfgaudit list --format json from the same release")
	out := flag.String("out", "_site", "output directory")
	flag.Parse()
	if err := build(Options{Src: *src, Version: *version, Reports: *reports, Rules: *rules, Out: *out}); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

// Options are the generator's inputs.
type Options struct {
	Src, Version, Reports, Rules, Out string
}

func build(o Options) error {
	src, version, out := o.Src, o.Version, o.Out
	if src == "" || !versionRe.MatchString(version) {
		return errors.New("-src and -version (X.Y.Z) are required")
	}
	inputs, reports, err := loadReports(o.Reports, version)
	if err != nil {
		return err
	}
	tmpl, err := template.New("").Funcs(template.FuncMap{"lower": strings.ToLower}).ParseFS(templateFS, "templates/*.html")
	if err != nil {
		return err
	}
	conv := converter{version: version}
	agents, err := loadAgents(src, conv)
	if err != nil {
		return err
	}

	ids, err := ruleIDs(src)
	if err != nil {
		return err
	}
	listed, err := loadListed(o.Rules, ids)
	if err != nil {
		return err
	}
	// A rule the list prints without a description gets the title of its doc.
	for id, r := range listed {
		if r.Description != "" {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(src, "docs", "rules", id+".md"))
		if err != nil {
			return err
		}
		first, _, _ := strings.Cut(string(raw), "\n")
		if m := ruleTitleRe.FindStringSubmatch(strings.TrimSpace(first)); m != nil {
			r.Description = strings.ReplaceAll(m[2], "`", "")
			listed[id] = r
		}
	}
	risks, covered, outside, err := sortRisks(llmRisks, listed, func(r ListedRule) []string { return []string{r.OWASP} })
	if err != nil {
		return err
	}
	mcpRisks, mcpCovered, _, err := sortRisks(mcpTop10, listed, func(r ListedRule) []string {
		if r.OWASPMCP == "" {
			return nil
		}
		return []string{r.OWASPMCP}
	})
	if err != nil {
		return err
	}
	if err := os.RemoveAll(out); err != nil {
		return err
	}

	var urls []string
	write := func(p Page, name string) error {
		p.Version, p.RuleCount = version, len(ids)
		dir := out
		if name != "404.html" {
			dir = filepath.Join(out, filepath.FromSlash(strings.Trim(p.Path, "/")))
		}
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
		var buf bytes.Buffer
		if err := tmpl.ExecuteTemplate(&buf, name, p); err != nil {
			return fmt.Errorf("%s: %w", p.Path, err)
		}
		if name != "404.html" {
			urls = append(urls, siteURL+p.Path)
		}
		file := "index.html"
		if name == "404.html" {
			file = "404.html"
		}
		return os.WriteFile(filepath.Join(dir, file), buf.Bytes(), 0o644)
	}

	if err := write(Page{
		Title:       "cfgaudit: static security auditor for AI-agent configuration",
		Description: fmt.Sprintf("cfgaudit finds risky permissions, hooks, MCP servers and instructions in Claude Code, Cursor, Codex, Copilot, Gemini CLI and other agent configuration before the agent runs: %d rules, offline, one binary.", len(ids)),
		Path:        "/",
		Sample:      shortLines(reports[0].Output),
		Covered:     covered,
		MCPCovered:  mcpCovered,
		Agents:      agents,
	}, "home.html"); err != nil {
		return err
	}

	if err := write(Page{
		Title:       "Example reports · cfgaudit",
		Description: "What cfgaudit's text, table, JSON, SARIF and Code Climate output look like, and where each one shows up.",
		Path:        "/reports/",
		Reports:     reports,
		Inputs:      inputs,
	}, "reports.html"); err != nil {
		return err
	}

	if err := write(Page{
		Title:       "OWASP Top 10 for LLM Applications and MCP · cfgaudit",
		Description: fmt.Sprintf("How cfgaudit's %d rules map to the OWASP Top 10 for LLM Applications 2026 and the OWASP MCP Top 10, and which risks a configuration file can't show.", len(ids)),
		Path:        "/owasp/",
		Risks:       risks,
		MCPRisks:    mcpRisks,
		Covered:     covered,
		Outside:     outside,
		MCPCovered:  mcpCovered,
	}, "owasp.html"); err != nil {
		return err
	}

	if err := write(Page{
		Title:       "Rules · cfgaudit",
		Description: fmt.Sprintf("All %d cfgaudit rules for AI-agent configuration, grouped by the OWASP Top 10 for LLM Applications.", len(ids)),
		Path:        "/rules/",
		Risks:       risks,
	}, "overview.html"); err != nil {
		return err
	}

	for _, id := range ids {
		file := "docs/rules/" + id + ".md"
		raw, err := os.ReadFile(filepath.Join(src, filepath.FromSlash(file)))
		if err != nil {
			return err
		}
		first, rest, _ := strings.Cut(string(raw), "\n")
		m := ruleTitleRe.FindStringSubmatch(strings.TrimSpace(first))
		if m == nil || m[1] != id {
			return fmt.Errorf("%s: first line %q is not \"# %s — <title>\" or \"# %s: <title>\"", file, first, id, id)
		}
		body, err := conv.convert(file, []byte(rest))
		if err != nil {
			return err
		}
		titleHTML, err := conv.convert(file, []byte(m[2]))
		if err != nil {
			return err
		}
		inline := strings.TrimSuffix(strings.TrimPrefix(strings.TrimSpace(string(titleHTML)), "<p>"), "</p>")
		r := &Rule{ID: id, Title: strings.ReplaceAll(m[2], "`", ""), TitleHTML: template.HTML(inline), Body: body, Listed: listed[id]} //nolint:gosec // goldmark output with raw HTML disabled
		if err := write(Page{
			Title:       id + ": " + r.Title + " · cfgaudit",
			Description: id + " " + r.Title + ": what cfgaudit reports, why it matters, and the safe alternative.",
			Path:        "/rules/" + id + "/",
			Rule:        r,
		}, "rule.html"); err != nil {
			return err
		}
	}

	if err := write(Page{Title: "Not found · cfgaudit", Path: "/404.html"}, "404.html"); err != nil {
		return err
	}
	if err := copyStatic(out); err != nil {
		return err
	}
	if err := writeSitemap(out, urls); err != nil {
		return err
	}
	return checkLinks(out)
}

// shortLines cuts each finding of the text output at its first " — ", and
// a finding still longer than 120 characters at a word, so the home page
// shows what was found and the reports page the full message. The summary
// line ("cfgaudit 1.18.1 — 10 findings") stays whole.
func shortLines(out string) []string {
	lines := strings.Split(out, "\n")
	for i, l := range lines {
		if strings.HasPrefix(l, "cfgaudit ") {
			continue
		}
		if head, _, ok := strings.Cut(l, " \u2014 "); ok {
			l = head
		}
		if r := []rune(l); len(r) > 120 {
			cut := string(r[:120])
			if j := strings.LastIndex(cut, " "); j > 0 {
				cut = cut[:j]
			}
			l = cut + " \u2026"
		}
		lines[i] = l
	}
	return lines
}

var hrefRe = regexp.MustCompile(`(?:href|src)="(/[^"#]*)`)

// checkLinks fails the build when a page links to a site path that has no file.
func checkLinks(out string) error {
	var broken []string
	err := filepath.WalkDir(out, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(p, ".html") {
			return err
		}
		data, err := os.ReadFile(p) //nolint:gosec // walks the generated site
		if err != nil {
			return err
		}
		for _, m := range hrefRe.FindAllStringSubmatch(string(data), -1) {
			target := filepath.Join(out, filepath.FromSlash(m[1]))
			if strings.HasSuffix(m[1], "/") {
				target = filepath.Join(target, "index.html")
			}
			if _, err := os.Stat(target); err != nil {
				rel, _ := filepath.Rel(out, p)
				broken = append(broken, rel+" -> "+m[1])
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	if len(broken) > 0 {
		return fmt.Errorf("broken links:\n  %s", strings.Join(broken, "\n  "))
	}
	return nil
}

// ruleIDs lists the rules that have a doc, sorted.
func ruleIDs(src string) ([]string, error) {
	entries, err := os.ReadDir(filepath.Join(src, "docs", "rules"))
	if err != nil {
		return nil, err
	}
	var ids []string
	for _, e := range entries {
		if m := ruleFileRe.FindStringSubmatch("docs/rules/" + e.Name()); m != nil {
			ids = append(ids, m[1])
		}
	}
	if len(ids) == 0 {
		return nil, fmt.Errorf("%s: no docs/rules/CFGxxx.md", src)
	}
	sort.Strings(ids)
	return ids, nil
}

type converter struct {
	version string
}

func newMarkdown(fix *linkFixer) goldmark.Markdown {
	// Raw HTML stays off (goldmark's default), so comments in the docs are
	// dropped and nothing in a doc can add markup to the page.
	return goldmark.New(
		goldmark.WithExtensions(extension.GFM),
		goldmark.WithParserOptions(
			parser.WithAutoHeadingID(),
			parser.WithASTTransformers(util.Prioritized(fix, 100)),
		),
	)
}

// convert renders one Markdown file of the cfgaudit repository.
func (c converter) convert(file string, raw []byte) (template.HTML, error) {
	fix := &linkFixer{file: file, version: c.version}
	var buf bytes.Buffer
	md := newMarkdown(fix)
	doc := md.Parser().Parse(text.NewReader(raw))
	if err := md.Renderer().Render(&buf, raw, doc); err != nil {
		return "", err
	}
	if fix.err != nil {
		return "", fix.err
	}
	return template.HTML(buf.String()), nil //nolint:gosec // goldmark output with raw HTML disabled
}

// linkFixer points relative links in a doc at the site: another rule doc
// becomes its page, and anything else the file on GitHub at the release tag.
type linkFixer struct {
	file    string
	version string
	err     error
}

func (f *linkFixer) Transform(doc *ast.Document, _ text.Reader, _ parser.Context) {
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if l, ok := n.(*ast.Link); ok && entering {
			dest, err := f.fix(string(l.Destination))
			if err != nil && f.err == nil {
				f.err = err
			}
			l.Destination = []byte(dest)
		}
		return ast.WalkContinue, nil
	})
}

func (f *linkFixer) fix(dest string) (string, error) {
	if dest == "" || strings.HasPrefix(dest, "#") || strings.Contains(dest, "://") || strings.HasPrefix(dest, "mailto:") {
		return dest, nil
	}
	target, frag, _ := strings.Cut(dest, "#")
	if frag != "" {
		frag = "#" + frag
	}
	resolved := path.Clean(path.Join(path.Dir(f.file), target))
	if strings.HasPrefix(resolved, "../") || resolved == ".." {
		return "", fmt.Errorf("%s: link %q leaves the repository", f.file, dest)
	}
	if m := ruleFileRe.FindStringSubmatch(resolved); m != nil {
		return "/rules/" + m[1] + "/" + frag, nil
	}
	return repoURL + "/blob/v" + f.version + "/" + resolved + frag, nil
}

func copyStatic(out string) error {
	return fs.WalkDir(staticFS, "static", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, err := staticFS.ReadFile(p)
		if err != nil {
			return err
		}
		dst := filepath.Join(out, filepath.FromSlash(strings.TrimPrefix(p, "static/")))
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		return os.WriteFile(dst, data, 0o644)
	})
}

func writeSitemap(out string, urls []string) error {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	b.WriteString(`<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">` + "\n")
	for _, u := range urls {
		fmt.Fprintf(&b, "  <url><loc>%s</loc></url>\n", template.HTMLEscapeString(u))
	}
	b.WriteString("</urlset>\n")
	if err := os.WriteFile(filepath.Join(out, "sitemap.xml"), []byte(b.String()), 0o644); err != nil {
		return err
	}
	robots := "User-agent: *\nAllow: /\nSitemap: " + siteURL + "/sitemap.xml\n"
	return os.WriteFile(filepath.Join(out, "robots.txt"), []byte(robots), 0o644)
}
