// Copyright the cfgaudit contributors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"errors"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"strings"
	"unicode"
)

// Agent is one row of the home page's agent list: an agent section under
// "What cfgaudit checks" at the release tag.
type Agent struct {
	Name   template.HTML // the heading before " — "
	Reads  template.HTML // what the heading names after " — "
	Anchor string        // the section's GitHub anchor in AgentsFile
}

// readmeTopics are the "What cfgaudit checks" sections that describe a
// surface shared by several agents rather than one agent.
var readmeTopics = map[string]bool{
	"`settings.json`":         true,
	"MCP servers":             true,
	"Instruction files":       true,
	"Plugin & skill packages": true,
	"Agent-skills lockfile":   true,
}

// agentsFiles are the files that hold "What cfgaudit checks", newest layout
// first: the rule index since the README was split (cfgaudit#1138), the
// README before it (#37).
var agentsFiles = []string{"docs/rules/README.md", "README.md"}

// checksSection returns the lines of md's "What cfgaudit checks" section,
// without its heading, and the heading's level. The section ends at the next
// heading of the same level or above; ok is false when md has no such heading.
// Lines inside a fenced code block are never headings: a YAML example's
// "# comment" must not end the section.
func checksSection(md string) (lines []string, level int, ok bool) {
	var fence fenceTracker
	for _, line := range strings.Split(md, "\n") {
		var l int
		var text string
		if !fence.inside(line) {
			l, text = headingLevel(line)
		}
		switch {
		case !ok && l > 0 && text == "What cfgaudit checks":
			level, ok = l, true
		case ok && l > 0 && l <= level:
			return lines, level, true
		case ok:
			lines = append(lines, line)
		}
	}
	return lines, level, ok
}

// fenceTracker follows ``` and ~~~ fences line by line.
type fenceTracker struct{ open string }

// inside reports whether line is part of a fenced code block, fence lines
// included.
func (f *fenceTracker) inside(line string) bool {
	t := strings.TrimSpace(line)
	switch {
	case f.open != "":
		if strings.HasPrefix(t, f.open) && strings.TrimLeft(t, f.open[:1]) == "" {
			f.open = ""
		}
		return true
	case strings.HasPrefix(t, "```"), strings.HasPrefix(t, "~~~"):
		f.open = t[:3]
		return true
	}
	return false
}

// headingLevel returns the level and text of an ATX heading line, or 0.
func headingLevel(line string) (int, string) {
	line = strings.TrimSpace(line)
	n := len(line) - len(strings.TrimLeft(line, "#"))
	if n == 0 || n > 6 || len(line) == n || line[n] != ' ' {
		return 0, ""
	}
	return n, strings.TrimSpace(line[n:])
}

// loadAgents reads the agent sections from the first of agentsFiles in the
// source tree, so a section added in a release appears on the site with it.
// It returns the file, for the links to the sections.
func loadAgents(src string, conv converter) ([]Agent, string, error) {
	for _, file := range agentsFiles {
		data, err := os.ReadFile(filepath.Join(src, filepath.FromSlash(file))) //nolint:gosec // src comes from the build script
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, "", err
		}
		agents, err := parseAgents(file, string(data), conv)
		return agents, file, err
	}
	return nil, "", fmt.Errorf("none of %s in %s", strings.Join(agentsFiles, ", "), src)
}

// parseAgents returns the sections one level below "What cfgaudit checks".
func parseAgents(file, md string, conv converter) ([]Agent, error) {
	lines, level, _ := checksSection(md)
	var agents []Agent
	var fence fenceTracker
	for _, line := range lines {
		if fence.inside(line) {
			continue
		}
		l, heading := headingLevel(line)
		if l != level+1 {
			continue
		}
		name, reads, _ := strings.Cut(heading, " — ")
		if readmeTopics[name] {
			continue
		}
		n, err := conv.inline(file, name)
		if err != nil {
			return nil, err
		}
		r, err := conv.inline(file, reads)
		if err != nil {
			return nil, err
		}
		agents = append(agents, Agent{Name: n, Reads: r, Anchor: githubAnchor(heading)})
	}
	if len(agents) == 0 {
		return nil, fmt.Errorf("%s: no agent sections under \"What cfgaudit checks\"", file)
	}
	return agents, nil
}

// inline renders a one-line Markdown fragment without the paragraph around it.
func (c converter) inline(file, md string) (template.HTML, error) {
	if strings.TrimSpace(md) == "" {
		return "", nil
	}
	h, err := c.convert(file, []byte(md))
	if err != nil {
		return "", fmt.Errorf("%s: %w", file, err)
	}
	s := strings.TrimSpace(string(h))
	return template.HTML(strings.TrimSuffix(strings.TrimPrefix(s, "<p>"), "</p>")), nil //nolint:gosec // goldmark output with raw HTML disabled
}

// githubAnchor is the id GitHub gives a Markdown heading: lower case, letters,
// digits, spaces, hyphens and underscores kept, everything else dropped, and
// spaces turned into hyphens.
func githubAnchor(heading string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(heading) {
		switch {
		case unicode.IsLetter(r), unicode.IsDigit(r), r == '-', r == '_':
			b.WriteRune(r)
		case r == ' ':
			b.WriteByte('-')
		}
	}
	return b.String()
}
