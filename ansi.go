// Copyright the cfgaudit contributors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"html"
	"html/template"
	"regexp"
	"strings"
)

var (
	// sgrRe matches an SGR escape sequence such as "\x1b[1m" or "\x1b[0;31m".
	sgrRe = regexp.MustCompile(`\x1b\[([0-9;]*)m`)
	// otherEscRe matches any other escape sequence, which is dropped.
	otherEscRe = regexp.MustCompile(`\x1b(\[[0-9;?]*[A-Za-z]|[^\[])`)
	// anyEscRe matches every escape sequence, for measuring visible text.
	anyEscRe = regexp.MustCompile(`\x1b(\[[0-9;?]*[A-Za-z]|[^\[])`)
)

// ansiStyle is the formatting cfgaudit's terminal output uses (#9): bold, and
// red, yellow or cyan for error, warn and info.
type ansiStyle struct {
	bold  bool
	color string
}

func (s ansiStyle) classes() string {
	var c []string
	if s.bold {
		c = append(c, "ansi-bold")
	}
	if s.color != "" {
		c = append(c, "ansi-"+s.color)
	}
	return strings.Join(c, " ")
}

func (s ansiStyle) apply(params string) ansiStyle {
	if params == "" {
		return ansiStyle{}
	}
	for _, p := range strings.Split(params, ";") {
		switch p {
		case "0":
			s = ansiStyle{}
		case "1":
			s.bold = true
		case "22":
			s.bold = false
		case "31":
			s.color = "red"
		case "33":
			s.color = "yellow"
		case "36":
			s.color = "cyan"
		case "39":
			s.color = ""
		}
	}
	return s
}

// ansiLine turns one line of terminal output into HTML: the text is escaped,
// SGR codes become spans with ansi-* classes, and every other escape sequence
// is dropped. Each line opens and closes its own spans, starting from style,
// and the style at its end is returned for the next line.
func ansiLine(line string, style ansiStyle) (template.HTML, ansiStyle) {
	var b strings.Builder
	open := func() {
		if c := style.classes(); c != "" {
			b.WriteString(`<span class="` + c + `">`)
		}
	}
	closeSpan := func() {
		if style.classes() != "" {
			b.WriteString("</span>")
		}
	}
	open()
	rest := line
	for {
		loc := sgrRe.FindStringSubmatchIndex(rest)
		if loc == nil {
			b.WriteString(html.EscapeString(otherEscRe.ReplaceAllString(rest, "")))
			break
		}
		b.WriteString(html.EscapeString(otherEscRe.ReplaceAllString(rest[:loc[0]], "")))
		closeSpan()
		style = style.apply(rest[loc[2]:loc[3]])
		open()
		rest = rest[loc[1]:]
	}
	closeSpan()
	out := strings.ReplaceAll(b.String(), `<span class="ansi-bold"></span>`, "")
	for _, c := range []string{"red", "yellow", "cyan"} {
		out = strings.ReplaceAll(out, `<span class="ansi-`+c+`"></span>`, "")
		out = strings.ReplaceAll(out, `<span class="ansi-bold ansi-`+c+`"></span>`, "")
	}
	return template.HTML(out), style //nolint:gosec // text is escaped above; only fixed span tags are added
}

// ansiLines converts every line of out.
func ansiLines(lines []string) []template.HTML {
	res := make([]template.HTML, len(lines))
	var style ansiStyle
	for i, l := range lines {
		res[i], style = ansiLine(l, style)
	}
	return res
}

// ansiHTML converts a whole output block.
func ansiHTML(out string) template.HTML {
	lines := ansiLines(strings.Split(out, "\n"))
	parts := make([]string, len(lines))
	for i, l := range lines {
		parts[i] = string(l)
	}
	return template.HTML(strings.Join(parts, "\n")) //nolint:gosec // built from ansiLine
}

// visible returns s without escape sequences.
func visible(s string) string { return anyEscRe.ReplaceAllString(s, "") }
