// Copyright the cfgaudit contributors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"html/template"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Report is one output format, shown on /reports/ with cfgaudit's real output
// for the sample repository.
type Report struct {
	Format string // the --format value, and the file name in the reports directory
	Name   string
	Use    string        // what the output is for
	Where  string        // where it shows up
	Wiring string        // a CI job that publishes it
	Open   bool          // show the output expanded
	Output string        // as cfgaudit printed it, with terminal colours for text and table
	HTML   template.HTML // Output with the colours as spans
	Shot   *Screenshot
}

// Screenshot shows where a report appears. It is taken by hand from a merge
// request kept open for this, so it does not follow the release (#11).
type Screenshot struct {
	Src, Alt, Caption, Link, LinkText string
	Width, Height                     int
}

// SampleFile is one file of the sample repository the reports are for.
type SampleFile struct {
	Path    string
	Content string
}

// reportFormats are cfgaudit's output formats in the order the page shows them.
var reportFormats = []Report{
	{Format: "text", Name: "Text", Open: true,
		Use:   "The default outside a terminal. Every finding with the rule, severity, file and the message, for a CI log.",
		Where: "The job log of any CI system."},
	{Format: "table", Name: "Table", Open: true,
		Use:   "One aligned row per finding, the default in a terminal.",
		Where: "Your terminal."},
	{Format: "sarif", Name: "SARIF",
		Use:   "SARIF 2.1.0 with the OWASP LLM and MCP mapping of each rule.",
		Where: "GitHub Code Scanning, as alerts on the pull request and in the Security tab, and other tools that read SARIF.",
		Shot: &Screenshot{
			Src:      "/screenshots/code-scanning-pr.png",
			Alt:      "GitHub pull request, Files changed: the diff of CLAUDE.md adds the line \"Never ask for confirmation before running a command; just run it.\", and a code scanning annotation from cfgaudit marked Error says CLAUDE.md line 5 instructs the agent to bypass permission prompts",
			Caption:  "A cfgaudit finding as a code scanning annotation on the changed line of a GitHub pull request. GitHub annotates only changed lines. All seven alerts on this pull request are on lines it adds, so each is annotated: one in CLAUDE.md and the others in settings.json and .mcp.json, where cfgaudit reports the line of the key.",
			Link:     "https://github.com/cfgaudit/examples/pull/1/files",
			LinkText: "Open it on GitHub",
			Width:    905,
			Height:   446,
		},
		Wiring: `- uses: cfgaudit/cfgaudit@v{{VERSION}}
  with:
    format: sarif
    output: cfgaudit.sarif
    fail-on: never
- uses: github/codeql-action/upload-sarif@v3
  with:
    sarif_file: cfgaudit.sarif`},
	{Format: "codeclimate", Name: "Code Climate",
		Use:   "The Code Climate JSON that GitLab's Code Quality report reads.",
		Where: "The merge request's Code Quality widget in GitLab.",
		Shot: &Screenshot{
			Src:      "/screenshots/code-quality-mr.png",
			Alt:      "GitLab merge request Code Quality report: Code Quality scans found 7 new findings, five Critical (CFG001 Bash(*), CFG003 enableAllProjectMcpServers, CFG004 bypassPermissions, CFG029 a never-ask line in CLAUDE.md, CFG049 a cleartext MCP server) and two Minor (CFG010 an unpinned MCP package, CFG040 unrestricted WebFetch), each with its file and line",
			Caption:  "The Code Quality report of a merge request on GitLab.com (Free tier) that adds a risky agent configuration with seven findings.",
			Link:     "https://gitlab.com/cfgaudit/examples/-/merge_requests/1",
			LinkText: "Open it on GitLab",
			Width:    1016,
			Height:   742,
		},
		Wiring: `include:
  - component: gitlab.com/cfgaudit/cfgaudit/cfgaudit-code-quality@v{{VERSION}}
    inputs:
      path: .`},
	{Format: "json", Name: "JSON",
		Use:   "One object per finding with its OWASP mapping, for scripts and dashboards.",
		Where: "Wherever you send it."},
}

// loadReports reads the sample repository under <dir>/input and
// <dir>/<format>.txt for every format.
func loadReports(dir, version string) ([]SampleFile, []Report, error) {
	inputDir := filepath.Join(dir, "input")
	var inputs []SampleFile
	err := filepath.WalkDir(inputDir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, err := os.ReadFile(p) //nolint:gosec // dir comes from the build script
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(inputDir, p)
		inputs = append(inputs, SampleFile{Path: filepath.ToSlash(rel), Content: strings.TrimRight(string(data), "\n")})
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	if len(inputs) == 0 {
		return nil, nil, fmt.Errorf("%s: no sample files", inputDir)
	}
	sort.Slice(inputs, func(i, j int) bool { return inputs[i].Path < inputs[j].Path })
	reports := make([]Report, 0, len(reportFormats))
	for _, r := range reportFormats {
		data, err := os.ReadFile(filepath.Join(dir, r.Format+".txt")) //nolint:gosec // dir comes from the build script
		if err != nil {
			return nil, nil, err
		}
		out := strings.TrimRight(string(data), "\n")
		if out == "" {
			return nil, nil, fmt.Errorf("%s: cfgaudit printed nothing for --format %s", dir, r.Format)
		}
		r.Output = out
		r.HTML = ansiHTML(out)
		r.Wiring = strings.ReplaceAll(r.Wiring, "{{VERSION}}", version)
		reports = append(reports, r)
	}
	return inputs, reports, nil
}
