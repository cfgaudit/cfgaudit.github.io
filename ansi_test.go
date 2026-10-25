// Copyright the cfgaudit contributors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"strings"
	"testing"
)

func TestAnsiLines(t *testing.T) {
	got := ansiLines([]string{
		"a [\x1b[33mwarn\x1b[0m] \x1b[1mCFG006\x1b[0m: <x> & y",
		"\x1b[36minfo\x1b[0m\x1b[2K done",
		"\x1b[1;31mstill",
		"open\x1b[0m",
	})
	want := []string{
		`a [<span class="ansi-yellow">warn</span>] <span class="ansi-bold">CFG006</span>: &lt;x&gt; &amp; y`,
		`<span class="ansi-cyan">info</span> done`,
		`<span class="ansi-bold ansi-red">still</span>`,
		`<span class="ansi-bold ansi-red">open</span>`,
	}
	for i := range want {
		if string(got[i]) != want[i] {
			t.Errorf("line %d: got %q, want %q", i, got[i], want[i])
		}
	}
}

func TestShortLinesKeepsColour(t *testing.T) {
	long := ".claude/settings.json [\x1b[31merror\x1b[0m] \x1b[1mCFG003\x1b[0m: " + strings.Repeat("word ", 40)
	l := shortLines(long)[0]
	if !strings.HasPrefix(l, ".claude/settings.json [\x1b[31merror\x1b[0m] \x1b[1mCFG003\x1b[0m: word") || !strings.HasSuffix(l, " …") {
		t.Errorf("got %q", l)
	}
	if n := len([]rune(visible(l))); n > 122 {
		t.Errorf("visible length %d", n)
	}
}
