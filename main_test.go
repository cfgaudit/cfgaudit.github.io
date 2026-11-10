// Copyright the cfgaudit contributors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
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
		"npx cfgaudit@1.18.1 .",
		"npm install --save-dev --save-exact cfgaudit@1.18.1",
		`<a href="https://www.npmjs.com/package/cfgaudit">npm</a>`,
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

// #37: once the cfgaudit README is split (cfgaudit#1138), the agent sections
// are read from docs/rules/README.md, one level below its "What cfgaudit
// checks" title, and linked there. The README's short section of the same
// name is not read, and a fenced "# comment" does not end the section.
func TestBuild_AgentsFromRuleIndex(t *testing.T) {
	dir := t.TempDir()
	if err := os.CopyFS(dir, os.DirFS("testdata")); err != nil {
		t.Fatal(err)
	}
	index := "# What cfgaudit checks\n\n## `settings.json` — permissions\n\n```yaml\n# not a heading\n```\n\n" +
		"## Gemini CLI — `.gemini/settings.json`\n\n### Nested — not an agent\n\n## Instruction files — `CLAUDE.md` & other agents\n"
	if err := os.WriteFile(filepath.Join(dir, "src", "docs", "rules", "README.md"), []byte(index), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "src", "README.md"), []byte("# cfgaudit\n\n## What cfgaudit checks\n\n### Stale — not read\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	o := fixture(filepath.Join(dir, "_site"))
	o.Src = filepath.Join(dir, "src")
	if err := build(o); err != nil {
		t.Fatal(err)
	}
	home := read(t, filepath.Join(o.Out, "index.html"))
	for _, want := range []string{
		`blob/v1.18.1/docs/rules/README.md#gemini-cli--geminisettingsjson">Gemini CLI</a></td><td><code>.gemini/settings.json</code></td>`,
		`blob/v1.18.1/docs/rules/README.md#what-cfgaudit-checks">Claude Code</a>`,
		`blob/v1.18.1/docs/rules/README.md#instruction-files--claudemd--other-agents">What cfgaudit checks</a>`,
	} {
		if !strings.Contains(home, want) {
			t.Errorf("home lacks %s", want)
		}
	}
	for _, bad := range []string{"Stale", "Nested", "blob/v1.18.1/README.md#"} {
		if strings.Contains(home, bad) {
			t.Errorf("home contains %q", bad)
		}
	}

	// A rule index without agent sections fails the build rather than
	// falling back to the README.
	if err := os.WriteFile(filepath.Join(dir, "src", "docs", "rules", "README.md"), []byte("# Rules\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := build(o); err == nil || !strings.Contains(err.Error(), "docs/rules/README.md: no agent sections") {
		t.Errorf("build: %v", err)
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

// #21, #46: the progress page charts the release history, the rules per
// agent and the scan time of each release from releases.jsonl, without a
// script.
func TestBuild_ProgressPage(t *testing.T) {
	o := fixture(filepath.Join(t.TempDir(), "_site"))
	o.Scans, o.Releases, o.History = "testdata/releases.jsonl", "testdata/releases.json", "testdata/history"
	o.Rules = withAgents(t, map[string][]string{"CFG001": {"claude-code"}, "CFG036": {"claude-code", "cursor"}})
	if err := build(o); err != nil {
		t.Fatal(err)
	}
	page := read(t, filepath.Join(o.Out, "progress", "index.html"))
	for _, want := range []string{
		"<title>Progress · cfgaudit</title>",
		// Release charts, oldest first, each point linking to its release.
		"The number of rules in each release, from 2026-05-25 to 2026-07-01.",
		`<a href="https://github.com/cfgaudit/cfgaudit/releases/tag/v1.1.0"><title>v1.1.0 · 2026-06-01 · 48 rules (+33)</title>`,
		`<title>v1.1.0 · 2026-06-01 · 4 agents (+3)</title>`,
		`<title>v1.2.0 · 2026-07-01 · 7 agents (+3)</title>`,
		// Rules per agent, longest bar first, from the agents field.
		`<span class="bar-name">Claude Code</span><span class="bar"><span style="width: 100%"></span></span><span class="bar-value">2</span>`,
		`<span class="bar-name">Cursor</span><span class="bar"><span style="width: 50%"></span></span><span class="bar-value">1</span>`,
		// The scan time of the latest run only (200), oldest release first,
		// each point linking to its release; a release without `list` has
		// no rule count.
		`<section id="performance">`,
		`<a href="https://github.com/cfgaudit/cfgaudit/releases/tag/v1.0.0"><title>v1.0.0 · 2026-05-25 · 76.9 ms</title>`,
		`<a href="https://github.com/cfgaudit/cfgaudit/releases/tag/v1.19.0"><title>v1.19.0 · 2026-10-20 · 618 ms · 116 rules</title>`,
		`<title>v1.20.0 · 2026-10-30 · 356 ms · 123 rules</title>`,
		`<tr><td><a href="https://github.com/cfgaudit/cfgaudit/releases/tag/v1.0.0">v1.0.0</a></td><td>2026-05-25</td><td>–</td><td>76.9</td></tr>`,
		`<a href="https://github.com/cfgaudit/cfgaudit/actions/runs/200">Release scan time</a> workflow on AMD EPYC 7763 64-Core Processor with go1.26.10.`,
		"3 times each after a warm-up scan",
	} {
		if !strings.Contains(page, want) {
			t.Errorf("progress page lacks %s", want)
		}
	}
	for _, bad := range []string{"<script", "Kimi CLI", "999", "Old CPU"} {
		if strings.Contains(page, bad) {
			t.Errorf("progress page contains %q", bad)
		}
	}
	if !strings.Contains(read(t, filepath.Join(o.Out, "index.html")), `<a href="/progress/">Progress</a>`) {
		t.Error("the footer does not link the progress page")
	}
	sitemap := read(t, filepath.Join(o.Out, "sitemap.xml"))
	if !strings.Contains(sitemap, "https://cfgaudit.github.io/progress/") || strings.Contains(sitemap, "/benchmarks/") {
		t.Errorf("sitemap:\n%s", sitemap)
	}
	// The old URL sends readers on and stays out of search results.
	old := read(t, filepath.Join(o.Out, "benchmarks", "index.html"))
	for _, want := range []string{`http-equiv="refresh" content="0; url=https://cfgaudit.github.io/progress/#performance"`, `<meta name="robots" content="noindex">`} {
		if !strings.Contains(old, want) {
			t.Errorf("redirect page lacks %s", want)
		}
	}
}

// withAgents writes a copy of the rule list fixture with agents added.
func withAgents(t *testing.T, agents map[string][]string) string {
	t.Helper()
	var listed []map[string]any
	if err := json.Unmarshal([]byte(read(t, "testdata/rules.json")), &listed); err != nil {
		t.Fatal(err)
	}
	for _, r := range listed {
		if a, ok := agents[r["id"].(string)]; ok {
			r["agents"] = a
		}
	}
	data, err := json.Marshal(listed)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "rules.json")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// A README covers an agent through a path only that agent reads, or its name
// where it has none. Table rows and passing mentions do not count.
func TestReadmeAgents(t *testing.T) {
	for _, c := range []struct {
		tag  string
		want string
	}{
		{"v1.0.0", "Claude Code"},
		{"v1.1.0", "Claude Code,Cursor,VS Code,Cline"},
		{"v1.2.0", "Claude Code,Cursor,Gemini CLI,goose,Crush,OpenHands,Roo Code"},
		{"v2.0.0", "Claude Code,Gemini CLI,qwen-code,Kimi CLI"},
	} {
		got := strings.Join(readmeAgents(read(t, filepath.Join("testdata", "history", "readmes", c.tag+".md"))), ",")
		if got != c.want {
			t.Errorf("%s: %s, want %s", c.tag, got, c.want)
		}
	}
}

// #48: New agents added in v1.21.0 are recognized by readmeAgents signals.
func TestReadmeAgents_V121(t *testing.T) {
	readme := `# cfgaudit

## What cfgaudit checks

### Junie
Checks rules in .junie/ directory.

### pi
Checks .pi/ directory.

### Kilo Code
Checks kilo.jsonc and .kilo/ or .kilocode/ configs.

### Antigravity CLI
Configured via .agents/ settings.
`
	got := strings.Join(readmeAgents(readme), ",")
	want := "Junie,pi,Kilo Code,Antigravity CLI"
	if got != want {
		t.Errorf("readmeAgents v1.21 = %q, want %q", got, want)
	}

	// Antigravity CLI matches only section headings, not inline mentions
	bodyOnly := `# cfgaudit

## What cfgaudit checks

Mentions Antigravity CLI in passing text without a section heading.
`
	if gotBody := strings.Join(readmeAgents(bodyOnly), ","); strings.Contains(gotBody, "Antigravity CLI") {
		t.Errorf("Antigravity CLI should not match inline mentions: %q", gotBody)
	}
}

// #48: All slugs from v1.21.0 have product names in agentNames.
func TestAgentNames_V121Slugs(t *testing.T) {
	required := map[string]string{
		"junie":          "Junie",
		"antigravity":    "Antigravity CLI",
		"pi":             "pi",
		"kilo-code":      "Kilo Code",
		"claude-desktop": "Claude Desktop",
	}
	for slug, want := range required {
		if got := agentName(slug); got != want {
			t.Errorf("agentName(%q) = %q, want %q", slug, got, want)
		}
	}
}

// #48: Every slug in cfgaudit list --format json must have a name,
// failing the build otherwise.
func TestLoadListed_RejectsUnknownAgentSlug(t *testing.T) {
	jsonPath := filepath.Join(t.TempDir(), "rules.json")
	content := `[{"id":"CFG001","owasp":"LLM03","agents":["unknown-agent-slug"]}]`
	if err := os.WriteFile(jsonPath, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadListed(jsonPath, []string{"CFG001"}); err == nil || !strings.Contains(err.Error(), "unknown agent") {
		t.Errorf("loadListed with unknown agent: want error mentioning unknown agent, got %v", err)
	}
}

// Without any data the site builds as before: no page and no link to it.
func TestBuild_NoProgressData(t *testing.T) {
	o := fixture(filepath.Join(t.TempDir(), "_site"))
	o.Scans, o.History = filepath.Join(t.TempDir(), "missing.jsonl"), filepath.Join(t.TempDir(), "missing")
	if err := build(o); err != nil {
		t.Fatal(err)
	}
	for _, dir := range []string{"progress", "benchmarks"} {
		if _, err := os.Stat(filepath.Join(o.Out, dir)); !os.IsNotExist(err) {
			t.Errorf("%s built without data: %v", dir, err)
		}
	}
	if strings.Contains(read(t, filepath.Join(o.Out, "index.html")), "/progress/") {
		t.Error("the footer links a page that does not exist")
	}
}

func TestLoadScans_RejectsOtherSchemas(t *testing.T) {
	for _, c := range []struct{ line, want string }{
		{`{"schema":2,"kind":"release","tag":"v1.0.0","median_ms":1}`, "schema 2"},
		{`{"schema":1,"kind":"main","tag":"v1.0.0","median_ms":1}`, `kind "main"`},
		{`{"schema":1,"kind":"release","tag":"v1.0.0","median_ms":0}`, "positive median_ms"},
		{`{"schema":1,`, "unexpected end of JSON input"},
	} {
		path := filepath.Join(t.TempDir(), "releases.jsonl")
		if err := os.WriteFile(path, []byte(c.line+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := loadScans(path); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: err = %v, want %q", c.line, err, c.want)
		}
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

func TestBuild_AdvisoriesPage(t *testing.T) {
	out := buildFixture(t)
	page := read(t, filepath.Join(out, "advisories", "index.html"))
	for _, want := range []string{
		`<a href="https://www.cve.org/CVERecord?id=CVE-2026-0001">CVE-2026-0001</a>`,
		`<a href="https://github.com/advisories/GHSA-aaaa-bbbb-cccc">GHSA-aaaa-bbbb-cccc</a>`,
		`<a href="/rules/CFG001/"><span class="rid">CFG001</span> <span class="sev">error</span></a>`,
		"fixed in 1.2.3",
		"<p>1 are detected, 1 partly, and 1 are not, each with the reason.",
		"A file scan cannot see this part",
		`<a href="https://github.com/cfgaudit/cfgaudit/issues/7">Issue #7</a>`,
		`id="some-campaign"`,
		`Payload runs: <a href="/rules/CFG004/">CFG004</a>`,
		"Package installs: A file scan cannot see this part",
	} {
		if !strings.Contains(page, want) {
			t.Errorf("advisories page lacks %s", want)
		}
	}
	if !strings.Contains(read(t, filepath.Join(out, "index.html")), `<a href="/advisories/">Attacks</a>`) {
		t.Error("the menu does not link the page")
	}
}

// A release from before docs/advisories.yml builds as before.
func TestBuild_NoAdvisories(t *testing.T) {
	src := filepath.Join(t.TempDir(), "src")
	if err := os.CopyFS(src, os.DirFS("testdata/src")); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(src, "docs", "advisories.yml")); err != nil {
		t.Fatal(err)
	}
	o := fixture(filepath.Join(t.TempDir(), "_site"))
	o.Src = src
	if err := build(o); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(o.Out, "advisories", "index.html")); err == nil {
		t.Error("page built without a list")
	}
	if strings.Contains(read(t, filepath.Join(o.Out, "index.html")), "/advisories/") {
		t.Error("the menu links a page that does not exist")
	}
}

func TestLoadAdvisories_Failures(t *testing.T) {
	listed := map[string]ListedRule{"CFG001": {}}
	for name, content := range map[string]string{
		"broken":  "{",
		"empty":   "[]\n",
		"unknown": "- {id: CVE-2026-1, kind: cve, coverage: full, rules: [{rule: CFG999, severity: error}]}\n",
	} {
		src := t.TempDir()
		path := filepath.Join(src, "docs", "advisories.yml")
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := loadAdvisories(src, listed); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
	if _, err := loadAdvisories(os.DevNull, listed); err == nil {
		t.Error("an unreadable source tree is not a missing list")
	}
}
