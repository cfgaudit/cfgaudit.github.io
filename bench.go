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

// The benchmarks page charts bench.jsonl from cfgaudit's bench-data branch
// (#17, cfgaudit/cfgaudit#1019): one line per push to main with the median of
// every benchmark unit and the time ratio against the latest release.
//
// The charts are inline SVG drawn here, so the page needs no script. Each
// chart shows one unit of one benchmark: never two scales on one axis. Time
// is shown only as the ratio against a release measured in the same job,
// because absolute times move with the runner hardware.

// benchLine is one line of bench.jsonl, format version 1.
type benchLine struct {
	V          int                            `json:"v"`
	SHA        string                         `json:"sha"`
	Date       string                         `json:"date"`
	Tag        string                         `json:"tag"`
	Go         string                         `json:"go"`
	CPU        string                         `json:"cpu"`
	Benchmarks map[string]map[string]*float64 `json:"benchmarks"`
}

// Release is a cfgaudit release as scripts/build.sh writes it: the tag, the
// commit it points at, when it was published and its release page.
type Release struct {
	Tag  string `json:"tag"`
	SHA  string `json:"sha"`
	Date string `json:"date"`
	URL  string `json:"url"`
}

// BenchPage is what the benchmarks template gets.
type BenchPage struct {
	Lines      int
	First      string // date of the first line
	Last       string // date of the last line
	LastSHA    string
	Benchmarks []BenchGroup
}

// BenchGroup is one benchmark: a chart per unit and the data as a table.
type BenchGroup struct {
	Name   string
	ID     string // HTML id
	Charts []Chart
	Units  []string
	Rows   []BenchRow // newest first
}

// BenchRow is one commit in a benchmark's data table.
type BenchRow struct {
	SHA, Short, Date string
	Values           []string // one per BenchGroup.Units
}

// Chart is one rendered SVG.
type Chart struct {
	Title string
	SVG   template.HTML
}

// chartUnits are the units the page charts, in this order. ns/op and
// init-ms/op are left out: they depend on the runner, and "ratio" carries
// time instead.
var chartUnits = []struct{ key, title string }{
	{"allocs/op", "Allocations per operation"},
	{"B/op", "Bytes allocated per operation"},
	{"init-allocs/op", "Allocations at package init"},
	{"ratio", "Time relative to the latest release"},
}

// loadBench reads bench.jsonl. A missing path or file gives nil: the site
// does not depend on the benchmark job.
func loadBench(path string) ([]benchLine, error) {
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
	defer func() { _ = f.Close() }()
	var lines []benchLine
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for n := 1; sc.Scan(); n++ {
		if strings.TrimSpace(sc.Text()) == "" {
			continue
		}
		var l benchLine
		if err := json.Unmarshal(sc.Bytes(), &l); err != nil {
			return nil, fmt.Errorf("%s:%d: %w", path, n, err)
		}
		if l.V != 1 {
			return nil, fmt.Errorf("%s:%d: format version %d, this generator reads 1", path, n, l.V)
		}
		// The runner reports its CPU model padded with spaces.
		l.CPU = strings.TrimSpace(l.CPU)
		if len(l.SHA) < 12 {
			return nil, fmt.Errorf("%s:%d: no commit SHA", path, n)
		}
		lines = append(lines, l)
	}
	return lines, sc.Err()
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

// marker is a vertical line on every chart: a release or a CPU change.
type marker struct {
	at    int // index into the lines
	label string
	title string
	url   string
	cpu   bool
}

// releaseMarkers places each release at the line of its commit or, when that
// commit has no line (the job runs on the push, often before the tag), at the
// first line after its publication. A release from before the first line gets
// no marker.
func releaseMarkers(lines []benchLine, releases []Release) []marker {
	bySHA := map[string]int{}
	for i, l := range lines {
		bySHA[l.SHA] = i
	}
	var out []marker
	for _, r := range releases {
		if i, ok := bySHA[r.SHA]; ok {
			out = append(out, marker{at: i, label: r.Tag, title: r.Tag, url: r.URL})
			continue
		}
		pub, err := time.Parse(time.RFC3339, r.Date)
		if err != nil {
			continue
		}
		// A release older than the first line predates the data: no marker.
		for i, l := range lines {
			if d, err := time.Parse(time.RFC3339, l.Date); err == nil && d.After(pub) {
				if i > 0 {
					out = append(out, marker{at: i, label: r.Tag, title: r.Tag + ", released before this commit; its own commit has no measurement", url: r.URL})
				}
				break
			}
		}
	}
	sort.Slice(out, func(a, b int) bool { return out[a].at < out[b].at })
	return out
}

func cpuMarkers(lines []benchLine) []marker {
	var out []marker
	for i := 1; i < len(lines); i++ {
		if lines[i].CPU != lines[i-1].CPU {
			out = append(out, marker{at: i, label: "CPU", title: "Runner CPU changed to " + lines[i].CPU + ": times before and after are not comparable", cpu: true})
		}
	}
	return out
}

func benchPage(lines []benchLine, releases []Release) *BenchPage {
	if len(lines) == 0 {
		return nil
	}
	markers := append(releaseMarkers(lines, releases), cpuMarkers(lines)...)
	names := map[string]bool{}
	for _, l := range lines {
		for b := range l.Benchmarks {
			names[b] = true
		}
	}
	sorted := make([]string, 0, len(names))
	for b := range names {
		sorted = append(sorted, b)
	}
	// Whole scans first, then the rule benchmarks, then startup.
	rank := func(b string) int {
		switch {
		case strings.HasPrefix(b, "Scan_"):
			return 0
		case strings.HasPrefix(b, "Run_"):
			return 1
		}
		return 2
	}
	sort.Slice(sorted, func(i, j int) bool {
		if ri, rj := rank(sorted[i]), rank(sorted[j]); ri != rj {
			return ri < rj
		}
		return sorted[i] < sorted[j]
	})

	p := &BenchPage{Lines: len(lines), First: day(lines[0].Date), Last: day(lines[len(lines)-1].Date), LastSHA: lines[len(lines)-1].SHA}
	for _, b := range sorted {
		g := BenchGroup{Name: b, ID: "b-" + strings.ToLower(strings.NewReplacer("/", "-", "_", "-").Replace(b))}
		for _, u := range chartUnits {
			pts := series(lines, b, u.key)
			if len(pts) == 0 {
				continue
			}
			g.Units = append(g.Units, u.key)
			g.Charts = append(g.Charts, Chart{Title: u.title, SVG: renderChart(pts, lines, markers, u.key, b+": "+u.title)})
		}
		if len(g.Charts) == 0 {
			continue
		}
		for i := len(lines) - 1; i >= 0 && len(g.Rows) < 30; i-- {
			l := lines[i]
			if l.Benchmarks[b] == nil {
				continue
			}
			row := BenchRow{SHA: l.SHA, Short: l.SHA[:7], Date: day(l.Date)}
			for _, u := range g.Units {
				row.Values = append(row.Values, formatValue(l.Benchmarks[b][u], u))
			}
			g.Rows = append(g.Rows, row)
		}
		p.Benchmarks = append(p.Benchmarks, g)
	}
	return p
}

type point struct {
	at   int
	v    float64
	line benchLine
}

func series(lines []benchLine, bench, unit string) []point {
	var pts []point
	for i, l := range lines {
		if v := l.Benchmarks[bench][unit]; v != nil {
			pts = append(pts, point{at: i, v: *v, line: l})
		}
	}
	return pts
}

func day(date string) string {
	if t, err := time.Parse(time.RFC3339, date); err == nil {
		return t.UTC().Format("2006-01-02")
	}
	return date
}

// formatValue prints a value the way a reader compares them: bytes in KiB or
// MiB, counts with a k or M suffix, ratios with two decimals.
func formatValue(v *float64, unit string) string {
	if v == nil {
		return "–"
	}
	x := *v
	switch {
	case unit == "ratio":
		return fmt.Sprintf("%.2f×", x)
	case strings.HasSuffix(unit, "B/op"):
		switch {
		case x >= 1<<20:
			return fmt.Sprintf("%.1f MiB", x/(1<<20))
		case x >= 1<<10:
			return fmt.Sprintf("%.1f KiB", x/(1<<10))
		}
		return fmt.Sprintf("%.0f B", x)
	case x >= 1e6:
		return fmt.Sprintf("%.2fM", x/1e6)
	case x >= 1e4:
		return strings.Replace(fmt.Sprintf("%.1fk", x/1e3), ".0k", "k", 1)
	}
	return fmt.Sprintf("%.0f", x)
}

// Chart geometry in viewBox units; the SVG scales to the column width.
const (
	chartW, chartH                 = 320.0, 150.0
	padL, padR, padT, padB         = 52.0, 10.0, 18.0, 22.0
	plotW, plotH                   = chartW - padL - padR, chartH - padT - padB
	pointR, hitR                   = 3.5, 11.0
	ratioFloor, ratioCeil, ratioOK = 0.5, 1.5, 1.0
)

func renderChart(pts []point, lines []benchLine, markers []marker, unit, label string) template.HTML {
	n := len(lines)
	lo, hi := 0.0, 0.0
	for _, p := range pts {
		hi = math.Max(hi, p.v)
	}
	if unit == "ratio" {
		// Centred on 1.0 so "same as the release" sits in the same place on
		// every ratio chart.
		lo, hi = ratioFloor, ratioCeil
		for _, p := range pts {
			lo, hi = math.Min(lo, p.v), math.Max(hi, p.v)
		}
	}
	if hi <= lo {
		hi = lo + 1
	}
	switch {
	case unit == "ratio":
	case strings.HasSuffix(unit, "B/op") && hi >= 1<<20:
		hi = niceCeil(hi/(1<<20)) * (1 << 20) // round in MiB, the unit the ticks print
	case strings.HasSuffix(unit, "B/op") && hi >= 1<<10:
		hi = niceCeil(hi/(1<<10)) * (1 << 10)
	default:
		hi = niceCeil(hi)
	}
	x := func(i int) float64 {
		if n <= 1 {
			return padL + plotW/2
		}
		return padL + plotW*float64(i)/float64(n-1)
	}
	y := func(v float64) float64 { return padT + plotH*(1-(v-lo)/(hi-lo)) }

	var b strings.Builder
	fmt.Fprintf(&b, `<svg class="chart" viewBox="0 0 %.0f %.0f" role="group" aria-label="%s">`, chartW, chartH, template.HTMLEscapeString(label))
	// Recessive grid: three lines with their values.
	for _, t := range []float64{lo, (lo + hi) / 2, hi} {
		fmt.Fprintf(&b, `<line class="grid" x1="%.1f" x2="%.1f" y1="%.1f" y2="%.1f"/>`, padL, padL+plotW, y(t), y(t))
		v := t
		fmt.Fprintf(&b, `<text class="tick" x="%.1f" y="%.1f">%s</text>`, padL-6, y(t)+3.5, template.HTMLEscapeString(formatValue(&v, unit)))
	}
	if unit == "ratio" {
		fmt.Fprintf(&b, `<line class="ref" x1="%.1f" x2="%.1f" y1="%.1f" y2="%.1f"><title>1.0: as fast as the release</title></line>`, padL, padL+plotW, y(ratioOK), y(ratioOK))
	}
	for _, m := range markers {
		cls := "release"
		if m.cpu {
			cls = "cpu"
		}
		fmt.Fprintf(&b, `<g class="%s"><title>%s</title><line x1="%.1f" x2="%.1f" y1="%.1f" y2="%.1f"/>`, cls, template.HTMLEscapeString(m.title), x(m.at), x(m.at), padT-4, padT+plotH)
		// Release labels sit above the plot, CPU labels inside its foot, so
		// a release and a CPU change on the same commit do not overlap.
		ly := padT - 6
		if m.cpu {
			ly = padT + plotH - 4
		}
		lab := fmt.Sprintf(`<text x="%.1f" y="%.1f">%s</text>`, x(m.at)+3, ly, template.HTMLEscapeString(m.label))
		if m.url != "" {
			lab = fmt.Sprintf(`<a href="%s">%s</a>`, template.HTMLEscapeString(m.url), lab)
		}
		b.WriteString(lab + `</g>`)
	}
	var path strings.Builder
	for i, p := range pts {
		cmd := "L"
		if i == 0 {
			cmd = "M"
		}
		fmt.Fprintf(&path, "%s%.1f %.1f ", cmd, x(p.at), y(p.v))
	}
	fmt.Fprintf(&b, `<path class="series" d="%s"/>`, strings.TrimSpace(path.String()))
	for i, p := range pts {
		tip := fmt.Sprintf("%s · %s · %s", p.line.SHA[:7], day(p.line.Date), formatValue(&p.v, unit))
		if i > 0 && pts[i-1].v != 0 {
			tip += fmt.Sprintf(" (%+.1f%% vs previous)", 100*(p.v-pts[i-1].v)/pts[i-1].v)
		}
		fmt.Fprintf(&b, `<a href="%s/commit/%s"><title>%s</title><circle class="hit" cx="%.1f" cy="%.1f" r="%.0f"/><circle class="pt" cx="%.1f" cy="%.1f" r="%.1f"/></a>`,
			repoURL, template.HTMLEscapeString(p.line.SHA), template.HTMLEscapeString(tip), x(p.at), y(p.v), hitR, x(p.at), y(p.v), pointR)
	}
	// The x axis spans every line, also where this series has no value yet.
	fmt.Fprintf(&b, `<text class="tick" x="%.1f" y="%.1f" text-anchor="start">%s</text>`, padL, chartH-6, day(lines[0].Date))
	if n > 1 {
		fmt.Fprintf(&b, `<text class="tick" x="%.1f" y="%.1f" text-anchor="end">%s</text>`, padL+plotW, chartH-6, day(lines[n-1].Date))
	}
	b.WriteString(`</svg>`)
	return template.HTML(b.String()) //nolint:gosec // every interpolated string is escaped above
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
