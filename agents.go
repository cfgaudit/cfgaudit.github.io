// Copyright the cfgaudit contributors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"errors"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"
)

// Agent is one row of the home page's agent list: an agent section under
// "What cfgaudit checks" in the README at the release tag.
type Agent struct {
	Name   template.HTML // the heading before " — "
	Reads  template.HTML // what the heading names after " — "
	Anchor string        // the README section's GitHub anchor
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

var headingRe = regexp.MustCompile(`^### (.+)$`)

// loadAgents reads the agent sections from README.md in the source tree, so a
// section added in a release appears on the site with it.
func loadAgents(src string, conv converter) ([]Agent, error) {
	data, err := os.ReadFile(filepath.Join(src, "README.md")) //nolint:gosec // src comes from the build script
	if err != nil {
		return nil, err
	}
	in := false
	var agents []Agent
	for _, line := range strings.Split(string(data), "\n") {
		switch {
		case strings.HasPrefix(line, "## "):
			in = strings.TrimSpace(line) == "## What cfgaudit checks"
			continue
		case !in:
			continue
		}
		m := headingRe.FindStringSubmatch(strings.TrimSpace(line))
		if m == nil {
			continue
		}
		name, reads, _ := strings.Cut(m[1], " — ")
		if readmeTopics[name] {
			continue
		}
		n, err := conv.inline("README.md", name)
		if err != nil {
			return nil, err
		}
		r, err := conv.inline("README.md", reads)
		if err != nil {
			return nil, err
		}
		agents = append(agents, Agent{Name: n, Reads: r, Anchor: githubAnchor(m[1])})
	}
	if len(agents) == 0 {
		return nil, errors.New("README.md: no agent sections under \"## What cfgaudit checks\"")
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
