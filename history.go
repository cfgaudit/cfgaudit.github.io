// Copyright the cfgaudit contributors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// The progress page (#21) shows how cfgaudit grows from release to release:
// its rules, the agents it covers, how the rules spread over those agents,
// and the benchmark history.

// historyEntry is one release as scripts/build.sh records it in history.json:
// the tag, when it was published, its release page, and the number of rule
// docs in its tree. The README at that tag sits beside it as readmes/<tag>.md.
type historyEntry struct {
	Tag   string `json:"tag"`
	Date  string `json:"date"`
	URL   string `json:"url"`
	Rules int    `json:"rules"`
}

// Release point on the progress charts.
type HistPoint struct {
	Tag, Date, URL string
	Rules          int
	Agents         []string // agent names the README at this tag covers
}

// AgentBar is one row of the rules-per-agent chart.
type AgentBar struct {
	Name  string
	Rules int
	Pct   float64 // of the longest bar, for the CSS width
}

// ProgressPage is what the progress template gets.
type ProgressPage struct {
	History    []HistPoint // oldest first
	RulesSVG   template.HTML
	AgentsSVG  template.HTML
	Bars       []AgentBar // empty until a release carries the agents field
	BarsTotal  int        // rules in that release
	Bench      *BenchPage // nil without benchmark data
	LatestTag  string
	FirstDate  string
	LatestDate string
}

// agentSignals tell which agents a README's "What cfgaudit checks" section
// covers. A path an agent alone reads is the most reliable signal, and a name
// only where an agent has no directory of its own. Table rows are skipped:
// the rule table names paths of agents cfgaudit does not read yet (CFG028
// lists .roo/). Agents that are mentioned without being covered (Roo Code,
// Kimi CLI) count only once a section heading names them.
var agentSignals = []struct {
	name     string
	patterns []string
	heading  bool // match section headings only
}{
	{"Claude Code", []string{`Claude Code`}, false},
	{"Cursor", []string{`\.cursor/`, `\.cursorrules`}, false},
	{"GitHub Copilot", []string{`copilot-instructions`, `\.github/copilot`, `\.github/hooks`}, false},
	{"VS Code", []string{`\.vscode/`}, false},
	{"OpenAI Codex CLI", []string{`\.codex/`}, false},
	{"Gemini CLI", []string{`\.gemini/`, `GEMINI\.md`}, false},
	{"qwen-code", []string{`\.qwen/`, `QWEN\.md`}, false},
	{"Cline", []string{`\.clinerules`, `\.cline/`, `cline_mcp_settings`}, false},
	{"Zed", []string{`\.zed/`, "Zed's `\\.rules`"}, false},
	{"Continue", []string{`\.continue/`}, false},
	{"Devin CLI", []string{`Devin CLI`}, false},
	{"Devin Desktop (formerly Windsurf)", []string{`\.windsurf`}, false},
	{"OpenCode", []string{`opencode\.json`, `\.opencode/`}, false},
	{"xAI Grok CLI", []string{`\.grok/`}, false},
	{"Kimi Code", []string{`\.kimi-code/`}, false},
	{"Amp", []string{`\bAmp\b`}, false},
	{"goose", []string{`\bgoose\b`}, false},
	{"Crush", []string{`\bcrush\b`}, false},
	{"OpenHands", []string{`OpenHands`}, false},
	{"Roo Code", []string{`Roo Code`}, true},
	{"Kimi CLI", []string{`Kimi CLI`}, true},
}

var compiledSignals = func() [][]*regexp.Regexp {
	out := make([][]*regexp.Regexp, len(agentSignals))
	for i, s := range agentSignals {
		for _, p := range s.patterns {
			out[i] = append(out[i], regexp.MustCompile(p))
		}
	}
	return out
}()

// readmeAgents returns the agents a README covers, by the signals above,
// read from its "What cfgaudit checks" section (the whole README when an
// older one has no such section).
func readmeAgents(readme string) []string {
	sec := readme
	if i := strings.Index(readme, "\n## What cfgaudit checks"); i >= 0 {
		sec = readme[i+1:]
		if j := strings.Index(sec[3:], "\n## "); j >= 0 {
			sec = sec[:j+3]
		}
	}
	var body, headings strings.Builder
	for _, l := range strings.Split(sec, "\n") {
		switch {
		case strings.HasPrefix(l, "|"):
		case strings.HasPrefix(l, "#"):
			headings.WriteString(l + "\n")
			body.WriteString(l + "\n")
		default:
			body.WriteString(l + "\n")
		}
	}
	var out []string
	for i, s := range agentSignals {
		text := body.String()
		if s.heading {
			text = headings.String()
		}
		for _, re := range compiledSignals[i] {
			if re.MatchString(text) {
				out = append(out, s.name)
				break
			}
		}
	}
	return out
}

// loadHistory reads history.json and the READMEs beside it. A missing
// directory gives nil, so the site builds without the history charts.
func loadHistory(dir string) ([]HistPoint, error) {
	if dir == "" {
		return nil, nil
	}
	raw, err := os.ReadFile(filepath.Join(dir, "history.json")) //nolint:gosec // path comes from the build script
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var entries []historyEntry
	if err := json.Unmarshal(raw, &entries); err != nil {
		return nil, fmt.Errorf("history.json: %w", err)
	}
	var out []HistPoint
	for _, e := range entries {
		if !strings.HasPrefix(e.Tag, "v") || strings.ContainsAny(e.Tag, `/\`) {
			return nil, fmt.Errorf("history.json: unexpected tag %q", e.Tag)
		}
		if _, err := time.Parse(time.RFC3339, e.Date); err != nil {
			return nil, fmt.Errorf("history.json: %s: %w", e.Tag, err)
		}
		readme, err := os.ReadFile(filepath.Join(dir, "readmes", e.Tag+".md")) //nolint:gosec // tag checked above
		if err != nil {
			return nil, fmt.Errorf("history: README for %s: %w", e.Tag, err)
		}
		out = append(out, HistPoint{Tag: e.Tag, Date: e.Date, URL: e.URL, Rules: e.Rules, Agents: readmeAgents(string(readme))})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Date < out[j].Date })
	return out, nil
}

// agentBars counts the rules per agent from the latest release's rule list,
// longest first. Rules carry no agents before cfgaudit#1036, and then the
// chart is left out rather than guessed.
func agentBars(listed map[string]ListedRule) []AgentBar {
	count := map[string]int{}
	for _, r := range listed {
		for _, a := range r.Agents {
			count[a]++
		}
	}
	if len(count) == 0 {
		return nil
	}
	var bars []AgentBar
	top := 0
	for a, n := range count {
		bars = append(bars, AgentBar{Name: agentName(a), Rules: n})
		top = max(top, n)
	}
	sort.Slice(bars, func(i, j int) bool {
		if bars[i].Rules != bars[j].Rules {
			return bars[i].Rules > bars[j].Rules
		}
		return bars[i].Name < bars[j].Name
	})
	for i := range bars {
		bars[i].Pct = math.Round(1000*float64(bars[i].Rules)/float64(top)) / 10
	}
	return bars
}

// agentNames are the product names of cfgaudit's agent slugs
// (knownAgents in cmd/cfgaudit/ruleagents.go). An unknown slug shows as is.
var agentNames = map[string]string{
	"amp": "Amp", "claude-code": "Claude Code", "cline": "Cline", "codex": "OpenAI Codex CLI",
	"continue": "Continue", "copilot": "GitHub Copilot", "crush": "Crush", "cursor": "Cursor",
	"devin": "Devin CLI", "devin-desktop": "Devin Desktop (formerly Windsurf)", "gemini-cli": "Gemini CLI",
	"goose": "goose", "grok": "xAI Grok CLI", "kimi-cli": "Kimi CLI", "kimi-code": "Kimi Code",
	"openhands": "OpenHands", "opencode": "OpenCode", "qwen-code": "qwen-code", "roo-code": "Roo Code",
	"vscode": "VS Code", "zed": "Zed",
}

func agentName(slug string) string {
	if n := agentNames[slug]; n != "" {
		return n
	}
	return slug
}

func progressPage(hist []HistPoint, listed map[string]ListedRule, bench *BenchPage) *ProgressPage {
	bars := agentBars(listed)
	if len(hist) == 0 && bench == nil && len(bars) == 0 {
		return nil
	}
	p := &ProgressPage{History: hist, Bars: bars, BarsTotal: len(listed), Bench: bench}
	if len(hist) > 0 {
		p.FirstDate, p.LatestDate, p.LatestTag = day(hist[0].Date), day(hist[len(hist)-1].Date), hist[len(hist)-1].Tag
		p.RulesSVG = renderStepChart(hist, func(h HistPoint) int { return h.Rules }, "rule", "Rules per release")
		p.AgentsSVG = renderStepChart(hist, func(h HistPoint) int { return len(h.Agents) }, "agent", "Agents covered per release")
	}
	return p
}

// renderStepChart draws one count per release as a step line over time: the
// count holds until the next release changes it. Each point links to its
// release notes.
func renderStepChart(hist []HistPoint, value func(HistPoint) int, noun, label string) template.HTML {
	const w, h, pl, pr, pt, pb = 640.0, 220.0, 44.0, 14.0, 14.0, 26.0
	pw, ph := w-pl-pr, h-pt-pb
	t0, _ := time.Parse(time.RFC3339, hist[0].Date)
	t1, _ := time.Parse(time.RFC3339, hist[len(hist)-1].Date)
	span := t1.Sub(t0).Seconds()
	x := func(d string) float64 {
		t, _ := time.Parse(time.RFC3339, d)
		if span <= 0 {
			return pl + pw/2
		}
		return pl + pw*t.Sub(t0).Seconds()/span
	}
	hi := 0
	for _, p := range hist {
		hi = max(hi, value(p))
	}
	// Four grid steps of a round size, so 116 rules top out at 150, not 200.
	step := niceCeil(float64(max(hi, 1)) / 4)
	top := step * math.Ceil(float64(max(hi, 1))/step)
	y := func(v float64) float64 { return pt + ph*(1-v/top) }

	var b strings.Builder
	fmt.Fprintf(&b, `<svg class="chart chart-wide" viewBox="0 0 %.0f %.0f" role="img" aria-label="%s">`, w, h, template.HTMLEscapeString(label))
	for t := 0.0; t <= top+step/2; t += step {
		fmt.Fprintf(&b, `<line class="grid" x1="%.1f" x2="%.1f" y1="%.1f" y2="%.1f"/><text class="tick" x="%.1f" y="%.1f">%.0f</text>`, pl, pl+pw, y(t), y(t), pl-6, y(t)+3.5, t)
	}
	var path strings.Builder
	for i, p := range hist {
		v := float64(value(p))
		if i == 0 {
			fmt.Fprintf(&path, "M%.1f %.1f", x(p.Date), y(v))
			continue
		}
		fmt.Fprintf(&path, " H%.1f V%.1f", x(p.Date), y(v))
	}
	fmt.Fprintf(&b, `<path class="series" d="%s"/>`, path.String())
	for i, p := range hist {
		v := value(p)
		tip := fmt.Sprintf("%s · %s · %d %s", p.Tag, day(p.Date), v, plural(noun, v))
		if i > 0 {
			if d := v - value(hist[i-1]); d != 0 {
				tip += fmt.Sprintf(" (%+d)", d)
			}
		}
		fmt.Fprintf(&b, `<a href="%s"><title>%s</title><circle class="hit" cx="%.1f" cy="%.1f" r="11"/><circle class="pt" cx="%.1f" cy="%.1f" r="3.5"/></a>`,
			template.HTMLEscapeString(p.URL), template.HTMLEscapeString(tip), x(p.Date), y(float64(v)), x(p.Date), y(float64(v)))
	}
	last := hist[len(hist)-1]
	fmt.Fprintf(&b, `<text class="direct" x="%.1f" y="%.1f">%d</text>`, x(last.Date)-4, y(float64(value(last)))-8, value(last))
	fmt.Fprintf(&b, `<text class="tick" x="%.1f" y="%.1f" text-anchor="start">%s</text><text class="tick" x="%.1f" y="%.1f" text-anchor="end">%s</text>`,
		pl, h-6, day(hist[0].Date), pl+pw, h-6, day(last.Date))
	b.WriteString(`</svg>`)
	return template.HTML(b.String()) //nolint:gosec // every interpolated string is escaped above
}

func plural(noun string, n int) string {
	if n == 1 {
		return noun
	}
	return noun + "s"
}
