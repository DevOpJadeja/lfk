package app

import (
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/janosmiko/lfk/internal/model"
)

// Context picker: a centered, filterable single-select overlay listing the
// kubeconfig contexts (opened by kb.ContextPicker, default "|"). Selecting a
// context hands off to executeContextCommand, the same path `:ctx <name>`
// takes, so both entry points switch contexts identically.

// openContextPicker populates the overlay with every known context and parks
// the cursor on the active one. Refused in union mode, mirroring `:ctx`.
func (m Model) openContextPicker() (tea.Model, tea.Cmd) {
	if m.unionMode {
		m.setStatusMessage(":ctx is disabled in union view", true)
		return m, scheduleStatusClear()
	}
	names := m.contextNames()
	if len(names) == 0 {
		m.setStatusMessage("No kube contexts found", true)
		return m, scheduleStatusClear()
	}
	current := m.activeContext()
	items := make([]model.Item, len(names))
	cursor := 0
	for i, name := range names {
		items[i] = model.Item{Name: name}
		if name == current {
			items[i].Status = "current"
			cursor = i
		}
	}
	m.overlayItems = items
	m.overlayFilter.Clear()
	m.contextPickerFilterMode = false
	m.pendingG = false
	m.overlayCursor = cursor
	m.overlay = overlayContextPicker
	return m, nil
}

// handleContextPickerOverlayKey routes keys to the filter input while it is
// active, otherwise to list navigation.
func (m Model) handleContextPickerOverlayKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if m.contextPickerFilterMode {
		return m.handleContextPickerFilterMode(msg)
	}
	return m.handleContextPickerNormalMode(msg)
}

func (m Model) handleContextPickerNormalMode(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	items := m.filteredOverlayItems()
	key := msg.String()
	if key != "g" {
		m.pendingG = false
	}
	switch key {
	case "esc", "q":
		if m.overlayFilter.Value != "" {
			m.overlayFilter.Clear()
			m.overlayCursor = 0
			return m, nil
		}
		return m.closeCurrentOverlay()
	case "enter":
		if m.overlayCursor < 0 || m.overlayCursor >= len(items) {
			return m, nil
		}
		return m.selectContextFromPicker(items[m.overlayCursor].Name)
	case "/":
		m.contextPickerFilterMode = true
		m.overlayFilter.Clear()
		m.overlayCursor = 0
		return m, nil
	case "g":
		if m.pendingG {
			m.pendingG = false
			m.overlayCursor = 0
			return m, nil
		}
		m.pendingG = true
		return m, nil
	}
	m.overlayCursor = contextPickerMove(key, m.overlayCursor, len(items))
	return m, nil
}

// contextPickerMove applies a cursor-motion key to cursor over n items.
// Unknown keys leave the cursor unchanged.
func contextPickerMove(key string, cursor, n int) int {
	last := n - 1
	switch key {
	case "j", "down", "ctrl+n":
		return clampOverlayCursor(cursor, 1, last)
	case "k", "up", "ctrl+p":
		return clampOverlayCursor(cursor, -1, last)
	case "ctrl+d", "shift+down":
		return clampOverlayCursor(cursor, 10, last)
	case "ctrl+u", "shift+up":
		return clampOverlayCursor(cursor, -10, last)
	case "ctrl+f", "pgdown":
		return clampOverlayCursor(cursor, 20, last)
	case "ctrl+b", "pgup":
		return clampOverlayCursor(cursor, -20, last)
	case "G", "end":
		return max(last, 0)
	case "home":
		return 0
	}
	return cursor
}

func (m Model) handleContextPickerFilterMode(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if isPaste(msg) {
		switch handlePastedText(&m.overlayFilter, []rune(msg.Text)) {
		case filterContinue:
			m.overlayCursor = 0
		case filterPasteMultiline:
			m.triggerPasteConfirm(strings.TrimRight(msg.Text, "\n"), pasteTargetOverlayFilter)
		}
		return m, nil
	}
	switch handleFilterKey(&m.overlayFilter, msg) {
	case filterEscape:
		m.contextPickerFilterMode = false
		m.overlayFilter.Clear()
		m.overlayCursor = 0
	case filterAccept:
		m.contextPickerFilterMode = false
		m.overlayCursor = 0
	case filterClose:
		return m.closeCurrentOverlay()
	case filterContinue:
		m.overlayCursor = 0
	}
	return m, nil
}

// selectContextFromPicker closes the picker and switches to name. Picking the
// context already in view is a no-op close; at the cluster picker no context
// is in view yet, so any pick (including the kubeconfig default) navigates.
func (m Model) selectContextFromPicker(name string) (tea.Model, tea.Cmd) {
	m.overlay = overlayNone
	m.overlayFilter.Clear()
	m.contextPickerFilterMode = false
	if m.nav.Level != model.LevelClusters && name == m.nav.Context {
		return m, nil
	}
	return m.executeContextCommand(name)
}
