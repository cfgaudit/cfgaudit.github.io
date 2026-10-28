// Copyright the cfgaudit contributors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func fixture(out string) Options {
	return Options{Src: "testdata/src", Version: "1.18.1", Reports: "testdata/reports", Rules: "testdata/rules.json", Out: out}
}

func buildFixture(t *testing.T) string {
	t.Helper()
	out := filepath.Join(t.TempDir(), "_site")
	if err := build(fixture(out)); err != nil {
		t.Fatal(err)
	}
	return out
}

func read(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path) //nolint:gosec // test reads its own output
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestBuild_Pages(t *testing.T) {
	out := buildFixture(t)
	for _, p := range []string{"index.html", "404.html", "owasp/index.html", "reports/index.html", "rules/index.html", "rules/CFG001/index.html", "rules/CFG110/index.html", "style.css", "favicon.ico", "favicon-32.png", "favicon-64.png", "apple-touch-icon.png", "logo.png", "screenshots/code-quality-mr.png", "screenshots/code-scanning-pr.png", "sitemap.xml", "robots.txt"} {
		if _, err := os.Stat(filepath.Join(out, p)); err != nil {
			t.Errorf("missing %s", p)
		}
	}
	sitemap := read(t, filepath.Join(out, "sitemap.xml"))
	if !strings.Contains(sitemap, "https://cfgaudit.github.io/rules/CFG110/") || strings.Contains(sitemap, "404") {
		t.Errorf("sitemap:\n%s", sitemap)
	}
}

func TestBuild_RulePage(t *testing.T) {
	out := buildFixture(t)
	page := read(t, filepath.Join(out, "rules", "CFG001", "index.html"))
	for _, want := range []string{
		"<title>CFG001: permissions.allow contains unrestricted Bash pattern · cfgaudit</title>",
		`<span class="rule-id">CFG001</span> <code>permissions.allow</code> contains unrestricted Bash pattern</h1>`,
		`href="/rules/CFG110/#safe-alternative"`,
		`href="https://github.com/cfgaudit/cfgaudit/blob/v1.18.1/README.md#usage"`,
		`href="https://github.com/cfgaudit/cfgaudit/blob/v1.18.1/rules/cfg001.go"`,
		`<link rel="canonical" href="https://cfgaudit.github.io/rules/CFG001/">`,
	} {
		if !strings.Contains(page, want) {
			t.Errorf("CFG001 page lacks %s", want)
		}
	}
	// Raw HTML in a doc never reaches the page.
	for _, bad := range []string{"<script", "# CFG001"} {
		if strings.Contains(page, bad) {
			t.Errorf("CFG001 page contains %q", bad)
		}
	}
	// The "# CFGxxx: title" form is a title too.
	if p := read(t, filepath.Join(out, "rules", "CFG110", "index.html")); !strings.Contains(p, `<span class="rule-id">CFG110</span> a settings env block injects code into every hook</h1>`) {
		t.Errorf("CFG110 title missing:\n%s", p)
	}
}

func TestBuild_HomeOverviewOWASP(t *testing.T) {
	out := buildFixture(t)
	home := read(t, filepath.Join(out, "index.html"))
	for _, want := range []string{
		// Cut at the first " — "; the summary line stays whole.
		`<span class="line" style="--i: 0">.claude/settings.json [<span class="ansi-red">error</span>] <span class="ansi-bold">CFG001</span>: permissions.allow contains &#34;Bash(*)&#34;</span>`,
		"cfgaudit 1.18.1 — 1 findings",
		"<strong>3/10</strong>OWASP LLM risks",
		"<strong>1/10</strong>OWASP MCP risks",
		"cfgaudit/cfgaudit@v1.18.1",
		"ghcr.io/cfgaudit/cfgaudit:1.18.1",
		"/plugin install cfgaudit@cfgaudit",
		"releases/download/v1.18.1/cfgaudit_1.18.1_linux_amd64.tar.gz",
	} {
		if !strings.Contains(home, want) {
			t.Errorf("home lacks %s", want)
		}
	}
	overview := read(t, filepath.Join(out, "rules", "index.html"))
	// A rule without a description in the list gets its doc title.
	if !strings.Contains(overview, `<a href="/rules/CFG110/">CFG110</a></td><td><span class="sev">error</span></td><td>a settings env block injects code into every hook</td>`) {
		t.Errorf("overview lacks CFG110 row:\n%s", overview)
	}
	owasp := read(t, filepath.Join(out, "owasp", "index.html"))
	for _, want := range []string{`id="llm03"`, `id="mcp04"`, "not in a configuration file"} {
		if !strings.Contains(owasp, want) {
			t.Errorf("owasp lacks %s", want)
		}
	}
	reports := read(t, filepath.Join(out, "reports", "index.html"))
	for _, want := range []string{"<code>.claude/settings.json</code>", "&lt;sarif&gt; &amp; output", "cfgaudit-code-quality@v1.18.1", `<img src="/screenshots/code-quality-mr.png"`, "https://gitlab.com/cfgaudit/examples/-/merge_requests/1", `<img src="/screenshots/code-scanning-pr.png"`, "Open it on GitHub", `[<span class="ansi-red">error</span>] <span class="ansi-bold">CFG001</span>`} {
		if !strings.Contains(reports, want) {
			t.Errorf("reports lack %s", want)
		}
	}
}

func TestBuild_Failures(t *testing.T) {
	for name, edit := range map[string]func(dir string) error{
		"bad title": func(dir string) error {
			return os.WriteFile(filepath.Join(dir, "src", "docs", "rules", "CFG001.md"), []byte("# CFG001 title without separator\n"), 0o600)
		},
		"link leaves repository": func(dir string) error {
			return os.WriteFile(filepath.Join(dir, "src", "docs", "rules", "CFG110.md"), []byte("# CFG110: t\n\n[x](../../../etc/passwd)\n"), 0o600)
		},
		"undocumented rule listed": func(dir string) error {
			return os.WriteFile(filepath.Join(dir, "rules.json"), []byte(`[{"id":"CFG001","severity":"error","owasp":"LLM03"},{"id":"CFG004","severity":"error","owasp":"LLM03"},{"id":"CFG036","severity":"error","owasp":"LLM02"},{"id":"CFG110","severity":"error","owasp":"LLM04"},{"id":"CFG999","severity":"error","owasp":"LLM01"}]`), 0o600)
		},
		"documented rule missing": func(dir string) error {
			return os.WriteFile(filepath.Join(dir, "rules.json"), []byte(`[{"id":"CFG001","severity":"error","owasp":"LLM03"}]`), 0o600)
		},
		"no agent sections": func(dir string) error {
			return os.WriteFile(filepath.Join(dir, "src", "README.md"), []byte("# cfgaudit\n"), 0o600)
		},
		"unknown category": func(dir string) error {
			return os.WriteFile(filepath.Join(dir, "rules.json"), []byte(`[{"id":"CFG001","severity":"error","owasp":"LLM03"},{"id":"CFG004","severity":"error","owasp":"LLM03"},{"id":"CFG036","severity":"error","owasp":"LLM02"},{"id":"CFG110","severity":"error","owasp":"LLM99"}]`), 0o600)
		},
	} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.CopyFS(dir, os.DirFS("testdata")); err != nil {
				t.Fatal(err)
			}
			if err := edit(dir); err != nil {
				t.Fatal(err)
			}
			o := Options{Src: filepath.Join(dir, "src"), Version: "1.18.1", Reports: filepath.Join(dir, "reports"), Rules: filepath.Join(dir, "rules.json"), Out: filepath.Join(dir, "_site")}
			if err := build(o); err == nil {
				t.Errorf("build succeeded")
			}
		})
	}
}

// #7: the home page lists the agent sections of the README at the release
// tag, and the topic sections and anything after the section are left out.
func TestBuild_Agents(t *testing.T) {
	home := read(t, filepath.Join(buildFixture(t), "index.html"))
	for _, want := range []string{
		`README.md#cursor--github-copilot--hooks-permissions-and-repository-settings">Cursor &amp; GitHub Copilot</a></td><td>hooks, permissions and repository settings</td>`,
		`README.md#openai-codex-cli--codexconfigtoml--agentsmd">OpenAI Codex CLI</a></td><td><code>.codex/config.toml</code> &amp; <code>AGENTS.md</code></td>`,
		`blob/v1.18.1/README.md#what-cfgaudit-checks">Claude Code</a>`,
	} {
		if !strings.Contains(home, want) {
			t.Errorf("home lacks %s", want)
		}
	}
	for _, bad := range []string{"settingsjson--permissions", "Instruction files</a>", "Not an agent"} {
		if strings.Contains(home, bad) {
			t.Errorf("home lists %q as an agent", bad)
		}
	}
}

func TestGithubAnchor(t *testing.T) {
	for in, want := range map[string]string{
		"Devin Desktop (formerly Windsurf) — `.devin/`, with `.windsurf/` as the fallback": "devin-desktop-formerly-windsurf--devin-with-windsurf-as-the-fallback",
		"qwen-code — `.qwen/` & `QWEN.md`":                                                 "qwen-code--qwen--qwenmd",
	} {
		if got := githubAnchor(in); got != want {
			t.Errorf("githubAnchor(%q) = %q, want %q", in, got, want)
		}
	}
}

// #17: the benchmarks page charts bench.jsonl from cfgaudit's bench-data
// branch, without a script.
func TestBuild_BenchmarksPage(t *testing.T) {
	o := fixture(filepath.Join(t.TempDir(), "_site"))
	o.Bench, o.Releases = "testdata/bench.jsonl", "testdata/releases.json"
	if err := build(o); err != nil {
		t.Fatal(err)
	}
	page := read(t, filepath.Join(o.Out, "benchmarks", "index.html"))
	for _, want := range []string{
		"<title>Benchmarks · cfgaudit</title>",
		"4 commits from 2026-10-28 to 2026-11-03",
		`<section class="bench" id="b-scan-repo">`,
		`<section class="bench" id="b-init-rules">`,
		"Allocations at package init",
		// Each point links to its commit and names its value.
		`<a href="https://github.com/cfgaudit/cfgaudit/commit/` + strings.Repeat("b", 40) + `"><title>bbbbbbb · 2026-10-29 · 13.5k (-82.9% vs previous)</title>`,
		// v1.20.0's commit has no line, so it sits at the first line after it.
		`<a href="https://github.com/cfgaudit/cfgaudit/releases/tag/v1.20.0"><text`,
		"v1.20.0, released before this commit; its own commit has no measurement",
		// The CPU changed between the second and third line.
		"Runner CPU changed to AMD EPYC 9V74 80-Core Processor",
		"0.95×",
	} {
		if !strings.Contains(page, want) {
			t.Errorf("benchmarks page lacks %s", want)
		}
	}
	// Scan_Repo comes before Init_Rules; v1.19.0 predates the data and has no marker.
	if strings.Index(page, `id="b-scan-repo"`) > strings.Index(page, `id="b-init-rules"`) {
		t.Error("whole scans should come first")
	}
	for _, bad := range []string{"<script", "releases/tag/v1.19.0", "ns/op</th>"} {
		if strings.Contains(page, bad) {
			t.Errorf("benchmarks page contains %q", bad)
		}
	}
	// The ratio chart appears only where a ratio exists; both benchmarks have one.
	if n := strings.Count(page, "<figcaption>Time relative to the latest release"); n != 2 {
		t.Errorf("%d ratio charts, want 2", n)
	}
	if !strings.Contains(read(t, filepath.Join(o.Out, "index.html")), `<a href="/benchmarks/">Benchmarks</a>`) {
		t.Error("the footer does not link the benchmarks page")
	}
	if !strings.Contains(read(t, filepath.Join(o.Out, "sitemap.xml")), "https://cfgaudit.github.io/benchmarks/") {
		t.Error("the sitemap lacks the benchmarks page")
	}
}

// Without data the site builds as before: no page and no link to it.
func TestBuild_NoBenchData(t *testing.T) {
	o := fixture(filepath.Join(t.TempDir(), "_site"))
	o.Bench = filepath.Join(t.TempDir(), "missing.jsonl")
	if err := build(o); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(o.Out, "benchmarks")); !os.IsNotExist(err) {
		t.Errorf("benchmarks page built without data: %v", err)
	}
	if strings.Contains(read(t, filepath.Join(o.Out, "index.html")), "/benchmarks/") {
		t.Error("the footer links a page that does not exist")
	}
}

func TestLoadBench_RejectsOtherVersions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bench.jsonl")
	if err := os.WriteFile(path, []byte(`{"v":2,"sha":"`+strings.Repeat("a", 40)+`"}`+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadBench(path); err == nil || !strings.Contains(err.Error(), "format version 2") {
		t.Errorf("err = %v, want a format version error", err)
	}
}

// #19: the header's navigation rules must not reach the .toc chips, which are
// <nav> elements too; a bare "nav a:hover" turned their text white on a light
// chip.
func TestStyle_NavRulesScopedToHeader(t *testing.T) {
	css, err := staticFS.ReadFile("static/style.css")
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(string(css), "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "nav") || strings.Contains(line, ", nav ") {
			t.Errorf("unscoped nav selector: %s", line)
		}
	}
}
