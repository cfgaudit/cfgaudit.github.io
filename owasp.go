// Copyright the cfgaudit contributors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"strings"
)

const (
	llmBase = "https://github.com/GenAI-Security-Project/GenAI-LLM-Top10/blob/main/2026/final/"
	mcpBase = "https://owasp.org/www-project-mcp-top-10/"
)

// Risk is one OWASP category and the cfgaudit rules that report it.
type Risk struct {
	ID      string // "LLM01", "MCP01"
	Name    string
	URL     string
	Summary string // what the risk looks like in agent configuration
	Outside string // why a configuration file can't show it, for risks no rule covers
	Rules   []ListedRule
}

// ListedRule is one entry of cfgaudit list --format json.
type ListedRule struct {
	ID          string   `json:"id"`
	Severity    string   `json:"severity"`
	OWASP       string   `json:"owasp"`
	OWASPMCP    string   `json:"owasp_mcp"`
	Description string   `json:"description"`
	Agents      []string `json:"agents"` // since cfgaudit#1036; empty before
}

// llmRisks are the OWASP Top 10 for LLM Applications 2026 in OWASP's order,
// cfgaudit's primary mapping. The summaries describe them in terms of a
// committed agent configuration; OWASP's pages have the full text.
var llmRisks = []Risk{
	{ID: "LLM01", Name: "Prompt Injection", URL: llmBase + "LLM01_PromptInjection.md",
		Summary: "Text the agent reads as trusted context tells it to do something else: override phrases, hidden Unicode, fake turn boundaries and concealment orders in CLAUDE.md, AGENTS.md, rules files, skills and commands."},
	{ID: "LLM02", Name: "Sensitive Information Disclosure", URL: llmBase + "LLM02_SensitiveInformationDisclosure.md",
		Summary: "Secrets and private files reach the model or leave the machine: credentials committed in settings and MCP configs, missing deny rules for .env and keys, commands that read credential files or send data to a remote host."},
	{ID: "LLM03", Name: "Excessive Agency", URL: llmBase + "LLM03_ExcessiveAgency.md",
		Summary: "The agent can do more than the task needs without asking: unrestricted shell and write permissions, bypass and YOLO modes, auto-approved MCP servers, hooks that run on every session, instructions not to ask for confirmation."},
	{ID: "LLM04", Name: "Supply Chain", URL: llmBase + "LLM04_SupplyChain.md",
		Summary: "Code the agent runs can change under it: unpinned MCP packages and images, plugins and marketplaces controlled by a committed file, remote installers piped into a shell, skills that ask to run a downloaded archive."},
	{ID: "LLM05", Name: "Data and Model Poisoning", URL: llmBase + "LLM05_DataModelPoisoning.md",
		Outside: "Training data, fine-tuning sets and model weights are not part of an agent's configuration files."},
	{ID: "LLM06", Name: "Unbounded Consumption", URL: llmBase + "LLM06_UnboundedConsumption.md",
		Outside: "Rate limits, quotas and spending caps are set at the model provider or gateway, not in the repository."},
	{ID: "LLM07", Name: "Misinformation", URL: llmBase + "LLM07_Misinformation.md",
		Outside: "Whether an answer is correct is a property of the model's output at run time, not of the files that configure the agent."},
	{ID: "LLM08", Name: "Hidden Context Exposure", URL: llmBase + "LLM08_HiddenContextExposure.md",
		Outside: "What a model reveals of its hidden context happens in a conversation, after the configuration has been read."},
	{ID: "LLM09", Name: "Vector and Embedding Weaknesses", URL: llmBase + "LLM09_VectorAndEmbeddingWeaknesses.md",
		Outside: "Vector stores and retrieval pipelines live in the application, not in the agent's configuration files."},
	{ID: "LLM10", Name: "Improper Output Handling", URL: llmBase + "LLM10_ImproperOutputHandling.md",
		Outside: "How an application handles model output is in its code, which cfgaudit does not analyse."},
}

// mcpTop10 is the OWASP MCP Top 10 (2025), cfgaudit's secondary mapping for
// MCP-related rules.
var mcpTop10 = []Risk{
	{ID: "MCP01", Name: "Token Mismanagement & Secret Exposure", URL: mcpBase,
		Summary: "Tokens and keys written into MCP server definitions, headers and environment blocks."},
	{ID: "MCP02", Name: "Privilege Escalation via Scope Creep", URL: mcpBase,
		Summary: "Servers that are approved wholesale or get more access than one task needs."},
	{ID: "MCP03", Name: "Tool Poisoning", URL: mcpBase,
		Summary: "Tool and server metadata that carries instructions for the model."},
	{ID: "MCP04", Name: "Software Supply Chain Attacks & Dependency Tampering", URL: mcpBase,
		Summary: "Server packages and images that are not pinned and can change after review."},
	{ID: "MCP05", Name: "Command Injection & Execution", URL: mcpBase,
		Summary: "Server commands that are inline shell scripts or interpreters running code from their arguments."},
	{ID: "MCP06", Name: "Prompt Injection via Contextual Payloads", URL: mcpBase,
		Summary: "Content the server returns that the agent is told to trust or act on."},
	{ID: "MCP07", Name: "Insufficient Authentication & Authorization", URL: mcpBase,
		Summary: "Remote servers reached without authentication or over plain HTTP."},
	{ID: "MCP08", Name: "Lack of Audit and Telemetry", URL: mcpBase,
		Outside: "Audit logs and telemetry of MCP calls are produced at run time, not declared in a configuration file."},
	{ID: "MCP09", Name: "Shadow MCP Servers", URL: mcpBase,
		Summary: "Servers that take over a trusted name or are added without review."},
	{ID: "MCP10", Name: "Context Injection & Over-Sharing", URL: mcpBase,
		Outside: "What context a server receives during a session depends on the conversation, not on the committed configuration."},
}

// loadListed reads cfgaudit list --format json. Every listed rule must have a
// doc page and every documented rule must be listed, so the site can't drift
// from the release.
func loadListed(path string, docIDs []string) (map[string]ListedRule, error) {
	data, err := os.ReadFile(path) //nolint:gosec // path comes from the build script
	if err != nil {
		return nil, err
	}
	var listed []ListedRule
	if err := json.Unmarshal(data, &listed); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	byID := map[string]ListedRule{}
	for _, r := range listed {
		if !slices.Contains(docIDs, r.ID) {
			return nil, fmt.Errorf("%s: rule %s has no docs/rules/%s.md", path, r.ID, r.ID)
		}
		if r.OWASP == "" {
			return nil, fmt.Errorf("%s: rule %s has no OWASP category", path, r.ID)
		}
		byID[r.ID] = r
	}
	for _, id := range docIDs {
		if _, ok := byID[id]; !ok {
			return nil, fmt.Errorf("%s: documented rule %s is missing", path, id)
		}
	}
	return byID, nil
}

// sortRisks sorts the rules into a copy of risks by the categories cats
// returns for each, and counts the risks with rules and those outside a
// configuration file. A category that isn't in risks fails the build.
func sortRisks(list []Risk, listed map[string]ListedRule, cats func(ListedRule) []string) ([]Risk, int, int, error) {
	risks := slices.Clone(list)
	index := map[string]int{}
	for i, r := range risks {
		index[r.ID] = i
	}
	for _, r := range listed {
		for _, cat := range cats(r) {
			i, ok := index[cat]
			if !ok {
				return nil, 0, 0, fmt.Errorf("rule %s names unknown OWASP category %q", r.ID, cat)
			}
			risks[i].Rules = append(risks[i].Rules, r)
		}
	}
	covered, outside := 0, 0
	for i := range risks {
		slices.SortFunc(risks[i].Rules, func(a, b ListedRule) int { return strings.Compare(a.ID, b.ID) })
		switch {
		case len(risks[i].Rules) > 0 && risks[i].Summary == "":
			return nil, 0, 0, fmt.Errorf("%s has rules but no summary", risks[i].ID)
		case len(risks[i].Rules) > 0:
			covered++
		case risks[i].Outside != "":
			outside++
		}
	}
	return risks, covered, outside, nil
}
