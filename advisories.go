// Copyright the cfgaudit contributors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// reasonTexts says in words why an advisory is not fully detected. The keys are
// the reason values of docs/advisories.yml.
var reasonTexts = map[string]string{
	"not-static":  "A file scan cannot see this part: it needs run-time state.",
	"no-rule-yet": "Visible in a file, but no rule covers it yet.",
	"by-design":   "Deliberately not reported.",
}

// Advisory is one entry of cfgaudit's docs/advisories.yml.
type Advisory struct {
	ID       string         `yaml:"id"`
	Kind     string         `yaml:"kind"`
	Agent    string         `yaml:"agent"`
	Fixed    string         `yaml:"fixed"`
	Summary  string         `yaml:"summary"`
	Coverage string         `yaml:"coverage"`
	Rules    []AdvisoryRule `yaml:"rules"`
	Reason   string         `yaml:"reason"`
	Issue    int            `yaml:"issue"`
	Note     string         `yaml:"note"`
	Steps    []AdvisoryStep `yaml:"steps"`
}

// AdvisoryRule is a rule and the severity its fixture gets.
type AdvisoryRule struct {
	Rule     string `yaml:"rule"`
	Severity string `yaml:"severity"`
}

// AdvisoryStep is one step of a campaign: the rules that see it, or why none do.
type AdvisoryStep struct {
	Step   string   `yaml:"step"`
	Rules  []string `yaml:"rules"`
	Reason string   `yaml:"reason"`
}

// Anchor is the entry's fragment on the page.
func (a Advisory) Anchor() string {
	return strings.ToLower(strings.ReplaceAll(a.ID, " ", "-"))
}

// Link is where the advisory is published; a campaign has no such page.
func (a Advisory) Link() string {
	switch {
	case strings.HasPrefix(a.ID, "CVE-"):
		return "https://www.cve.org/CVERecord?id=" + a.ID
	case strings.HasPrefix(a.ID, "GHSA-"):
		return "https://github.com/advisories/" + a.ID
	}
	return ""
}

// ReasonText is Reason in words.
func (a Advisory) ReasonText() string { return reasonTexts[a.Reason] }

// ReasonText is Reason in words.
func (s AdvisoryStep) ReasonText() string { return reasonTexts[s.Reason] }

// AdvisoryPage is the /advisories/ page: a nil page means the release has no
// docs/advisories.yml, so the site has neither the page nor the menu entry.
type AdvisoryPage struct {
	Detected, Missing, Campaigns []Advisory
	Full, Partial, None          int
}

// loadAdvisories reads docs/advisories.yml of the release. A rule the file names
// has to exist in that release, since the page links it.
func loadAdvisories(src string, listed map[string]ListedRule) (*AdvisoryPage, error) {
	const file = "docs/advisories.yml"
	raw, err := os.ReadFile(filepath.Join(src, filepath.FromSlash(file)))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil //nolint:nilnil // no list is a valid state for an older release
	}
	if err != nil {
		return nil, err
	}
	var all []Advisory
	if err := yaml.Unmarshal(raw, &all); err != nil {
		return nil, fmt.Errorf("%s: %w", file, err)
	}
	if len(all) == 0 {
		return nil, fmt.Errorf("%s: no advisories", file)
	}
	page := &AdvisoryPage{}
	for _, a := range all {
		for _, r := range a.Rules {
			if _, ok := listed[r.Rule]; !ok {
				return nil, fmt.Errorf("%s: %s names unknown rule %s", file, a.ID, r.Rule)
			}
		}
		switch {
		case a.Kind == "campaign":
			page.Campaigns = append(page.Campaigns, a)
		case a.Coverage == "none":
			page.Missing = append(page.Missing, a)
			page.None++
		case a.Coverage == "full":
			page.Detected = append(page.Detected, a)
			page.Full++
		default:
			page.Detected = append(page.Detected, a)
			page.Partial++
		}
	}
	return page, nil
}
