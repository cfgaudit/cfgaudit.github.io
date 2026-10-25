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
	for _, p := range []string{"index.html", "404.html", "owasp/index.html", "reports/index.html", "rules/index.html", "rules/CFG001/index.html", "rules/CFG110/index.html", "style.css", "favicon.ico", "favicon-32.png", "favicon-64.png", "apple-touch-icon.png", "logo.png", "screenshots/code-quality-mr.png", "sitemap.xml", "robots.txt"} {
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
	for _, want := range []string{"<code>.claude/settings.json</code>", "&lt;sarif&gt; &amp; output", "cfgaudit-code-quality@v1.18.1", `<img src="/screenshots/code-quality-mr.png"`, "https://gitlab.com/cfgaudit/examples/-/merge_requests/1", `[<span class="ansi-red">error</span>] <span class="ansi-bold">CFG001</span>`} {
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
