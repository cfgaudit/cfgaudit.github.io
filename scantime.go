// Copyright the cfgaudit contributors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"math"
	"os"
	"sort"
	"strings"
	"time"
)

// The performance section of the progress page (#46) charts releases.jsonl
// from cfgaudit's bench-data branch (cfgaudit/cfgaudit#1619): the scan time of
// every release on one generated input. Only the latest run is drawn, because
// a run measures all releases in one job, taking turns, so its points compare
// with each other; times from different runs depend on different runners.

// scanRecord is one line of releases.jsonl, schema 1.
type scanRecord struct {
	Schema   int       `json:"schema"`
	Kind     string    `json:"kind"`
	Tag      string    `json:"tag"`
	Commit   string    `json:"commit"`
	Date     string    `json:"date"`
	Rules    *int      `json:"rules"`
	Input    string    `json:"input"`
	Run      int64     `json:"run"`
	MedianMS float64   `json:"median_ms"`
	RunsMS   []float64 `json:"runs_ms"`
	CPU      string    `json:"cpu"`
	Go       string    `json:"go"`
}

// Release is a cfgaudit release as scripts/build.sh writes it: the tag, the
// commit it points at, when it was published and its release page.
type Release struct {
	Tag  string `json:"tag"`
	SHA  string `json:"sha"`
	Date string `json:"date"`
	URL  string `json:"url"`
}

// ScanPage is what the performance section of the progress template gets.
type ScanPage struct {
	SVG     template.HTML
	Rows    []ScanRow // newest first
	Run     int64
	CPU, Go string
	Input   string
	Runs    int // timed scans per release
}

// ScanRow is one release in the data table.
type ScanRow struct {
	Tag, URL, Date string
	Rules          string // "–" where the release cannot list its rules
	MedianMS       string
}

func loadScans(path string) ([]scanRecord, error) {
	if path == "" {
		return nil, nil
	}
	f, err := os.Open(path) //nolint:gosec // path comes from the build script
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close() //nolint:errcheck // read-only
	var recs []scanRecord
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
	for n := 1; sc.Scan(); n++ {
		if strings.TrimSpace(sc.Text()) == "" {
			continue
		}
		var r scanRecord
		if err := json.Unmarshal(sc.Bytes(), &r); err != nil {
			return nil, fmt.Errorf("%s:%d: %w", path, n, err)
		}
		if r.Schema != 1 || r.Kind != "release" {
			return nil, fmt.Errorf("%s:%d: schema %d, kind %q: this build reads schema 1, kind release", path, n, r.Schema, r.Kind)
		}
		if r.Tag == "" || r.MedianMS <= 0 {
			return nil, fmt.Errorf("%s:%d: a record needs a tag and a positive median_ms", path, n)
		}
		recs = append(recs, r)
	}
	return recs, sc.Err()
}

func loadReleases(path string) ([]Release, error) {
	if path == "" {
		return nil, nil
	}
	raw, err := os.ReadFile(path) //nolint:gosec // path comes from the build script
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var rs []Release
	if err := json.Unmarshal(raw, &rs); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return rs, nil
}

// latestRun keeps the records of the run with the highest id, oldest
// release first.
func latestRun(recs []scanRecord) []scanRecord {
	var run int64
	for _, r := range recs {
		run = max(run, r.Run)
	}
	var out []scanRecord
	for _, r := range recs {
		if r.Run == run {
			out = append(out, r)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Date < out[j].Date })
	return out
}

func scanPage(recs []scanRecord, releases []Release) *ScanPage {
	run := latestRun(recs)
	if len(run) == 0 {
		return nil
	}
	urls := map[string]string{}
	for _, r := range releases {
		urls[r.Tag] = r.URL
	}
	url := func(tag string) string {
		if u := urls[tag]; u != "" {
			return u
		}
		return "https://github.com/cfgaudit/cfgaudit/releases/tag/" + tag
	}
	last := run[len(run)-1]
	p := &ScanPage{Run: last.Run, CPU: last.CPU, Go: last.Go, Input: last.Input, Runs: len(last.RunsMS)}
	for i := len(run) - 1; i >= 0; i-- {
		r := run[i]
		rules := "–"
		if r.Rules != nil {
			rules = fmt.Sprint(*r.Rules)
		}
		p.Rows = append(p.Rows, ScanRow{Tag: r.Tag, URL: url(r.Tag), Date: day(r.Date), Rules: rules, MedianMS: formatMS(r.MedianMS)})
	}
	p.SVG = renderScanChart(run, url)
	return p
}

func formatMS(v float64) string {
	if v >= 100 {
		return fmt.Sprintf("%.0f", v)
	}
	return fmt.Sprintf("%.1f", v)
}

// renderScanChart draws the median of each release as a line, one release per
// step on the x axis in release order, so releases of the same day stay apart.
// Each point links to its release notes.
func renderScanChart(run []scanRecord, url func(string) string) template.HTML {
	const w, h, pl, pr, pt, pb = 640.0, 220.0, 44.0, 14.0, 14.0, 26.0
	pw, ph := w-pl-pr, h-pt-pb
	x := func(i int) float64 {
		if len(run) == 1 {
			return pl + pw/2
		}
		return pl + pw*float64(i)/float64(len(run)-1)
	}
	hi := 0.0
	for _, r := range run {
		hi = math.Max(hi, r.MedianMS)
	}
	step := niceCeil(math.Max(hi, 1) / 4)
	top := step * math.Ceil(math.Max(hi, 1)/step)
	y := func(v float64) float64 { return pt + ph*(1-v/top) }

	var b strings.Builder
	fmt.Fprintf(&b, `<svg class="chart chart-wide" viewBox="0 0 %.0f %.0f" role="group" aria-label="Scan time of each release, ms">`, w, h)
	for t := 0.0; t <= top+step/2; t += step {
		fmt.Fprintf(&b, `<line class="grid" x1="%.1f" x2="%.1f" y1="%.1f" y2="%.1f"/><text class="tick" x="%.1f" y="%.1f">%.0f</text>`, pl, pl+pw, y(t), y(t), pl-6, y(t)+3.5, t)
	}
	var path strings.Builder
	for i, r := range run {
		cmd := "L"
		if i == 0 {
			cmd = "M"
		}
		fmt.Fprintf(&path, "%s%.1f %.1f ", cmd, x(i), y(r.MedianMS))
	}
	fmt.Fprintf(&b, `<path class="series" d="%s"/>`, strings.TrimSpace(path.String()))
	for i, r := range run {
		tip := fmt.Sprintf("%s · %s · %s ms", r.Tag, day(r.Date), formatMS(r.MedianMS))
		if r.Rules != nil {
			tip += fmt.Sprintf(" · %d %s", *r.Rules, plural("rule", *r.Rules))
		}
		fmt.Fprintf(&b, `<a href="%s"><title>%s</title><circle class="hit" cx="%.1f" cy="%.1f" r="11"/><circle class="pt" cx="%.1f" cy="%.1f" r="3.5"/></a>`,
			template.HTMLEscapeString(url(r.Tag)), template.HTMLEscapeString(tip), x(i), y(r.MedianMS), x(i), y(r.MedianMS))
	}
	last := run[len(run)-1]
	fmt.Fprintf(&b, `<text class="direct" x="%.1f" y="%.1f">%s ms</text>`, x(len(run)-1)-4, y(last.MedianMS)-8, formatMS(last.MedianMS))
	fmt.Fprintf(&b, `<text class="tick" x="%.1f" y="%.1f" text-anchor="start">%s</text><text class="tick" x="%.1f" y="%.1f" text-anchor="end">%s</text>`,
		pl, h-6, template.HTMLEscapeString(run[0].Tag), pl+pw, h-6, template.HTMLEscapeString(last.Tag))
	b.WriteString(`</svg>`)
	return template.HTML(b.String()) //nolint:gosec // every interpolated string is escaped above
}

func day(date string) string {
	if t, err := time.Parse(time.RFC3339, date); err == nil {
		return t.UTC().Format("2006-01-02")
	}
	return date
}

// niceCeil rounds v up to 1, 2 or 5 times a power of ten, so the top grid line
// carries a round number.
func niceCeil(v float64) float64 {
	if v <= 0 {
		return 1
	}
	p := math.Pow(10, math.Floor(math.Log10(v)))
	for _, m := range []float64{1, 2, 5, 10} {
		if v <= m*p {
			return m * p
		}
	}
	return 10 * p
}
