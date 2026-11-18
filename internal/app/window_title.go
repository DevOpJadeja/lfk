package app

import (
	"strings"
	"unicode"
)

// contextLabel names the active context for the breadcrumb and the terminal
// window title: the context itself, the bracketed member list in union view,
// or "" on the cluster list.
func (m Model) contextLabel() string {
	if m.isUnionSentinel() {
		return "[" + strings.Join(m.unionContexts, "+") + "]"
	}
	if m.nav.Context == UnionContextSentinel {
		return ""
	}
	return m.nav.Context
}

// windowTitle is the terminal window title, "lfk - <context>". View() sets it
// on every render; Bubble Tea only writes it when it changes and clears it
// on exit.
func (m Model) windowTitle() string {
	if label := stripControl(m.contextLabel()); label != "" {
		return "lfk - " + label
	}
	return "lfk"
}

// stripControl drops control characters. The title is written verbatim into
// an OSC sequence, so a kubeconfig context name carrying BEL/ESC/ST could
// otherwise end it early and inject terminal escapes.
func stripControl(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, s)
}
