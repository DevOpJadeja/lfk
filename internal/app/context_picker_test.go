package app

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

// contextPickerModel returns a model on "test-ctx" whose client knows three
// contexts (alpha, test-ctx, zeta), with the context picker key bound to "|".
func contextPickerModel(t *testing.T) Model {
	t.Helper()
	prev := ui.ActiveKeybindings.ContextPicker
	ui.ActiveKeybindings.ContextPicker = "|"
	t.Cleanup(func() { ui.ActiveKeybindings.ContextPicker = prev })

	m := basePush80Model()
	m.client.AddTestContext("alpha", "https://alpha.example.local:6443")
	m.client.AddTestContext("zeta", "https://zeta.example.local:6443")
	return m
}

func openPicker(t *testing.T, m Model) Model {
	t.Helper()
	ret, _, handled := m.handleExplorerUIKey(keyMsg("|"))
	require.True(t, handled, "context picker key handled")
	return ret.(Model)
}

func pressOverlay(t *testing.T, m Model, keys ...string) (Model, tea.Cmd) {
	t.Helper()
	var cmd tea.Cmd
	for _, k := range keys {
		var ret tea.Model
		ret, cmd = m.handleOverlayKey(keyMsg(k))
		m = ret.(Model)
	}
	return m, cmd
}

func overlayNames(m Model) []string {
	items := m.filteredOverlayItems()
	names := make([]string, len(items))
	for i, it := range items {
		names[i] = it.Name
	}
	return names
}

func TestContextPicker_OpenListsContextsWithCursorOnCurrent(t *testing.T) {
	m := openPicker(t, contextPickerModel(t))

	assert.Equal(t, overlayContextPicker, m.overlay)
	assert.Equal(t, []string{"alpha", "test-ctx", "zeta"}, overlayNames(m))
	require.Equal(t, 1, m.overlayCursor, "cursor parks on the active context")
	assert.Equal(t, "current", m.overlayItems[1].Status)
	assert.Empty(t, m.overlayItems[0].Status)
}

func TestContextPicker_OpensAtClusterLevel(t *testing.T) {
	m := contextPickerModel(t)
	m.nav = model.NavigationState{Level: model.LevelClusters}
	m = openPicker(t, m)

	assert.Equal(t, overlayContextPicker, m.overlay)
	assert.Equal(t, 1, m.overlayCursor, "falls back to the kubeconfig current context")
}

func TestContextPicker_FilterNarrowsList(t *testing.T) {
	m := openPicker(t, contextPickerModel(t))
	m, _ = pressOverlay(t, m, "/", "z", "e")

	assert.True(t, m.contextPickerFilterMode)
	assert.Equal(t, []string{"zeta"}, overlayNames(m))

	m, _ = pressOverlay(t, m, "enter")
	assert.False(t, m.contextPickerFilterMode, "enter accepts the filter")
	assert.Equal(t, overlayContextPicker, m.overlay)
	assert.Equal(t, []string{"zeta"}, overlayNames(m))
}

func TestContextPicker_PipeInFilterDoesNotClose(t *testing.T) {
	m := openPicker(t, contextPickerModel(t))
	m, _ = pressOverlay(t, m, "/", "a", "|")

	assert.Equal(t, overlayContextPicker, m.overlay, "| is filter text while filtering")
	assert.Equal(t, "a|", m.overlayFilter.Value)
}

func TestContextPicker_EnterSwitchesContext(t *testing.T) {
	m := openPicker(t, contextPickerModel(t))
	m, cmd := pressOverlay(t, m, "G", "enter")

	assert.Equal(t, overlayNone, m.overlay)
	assert.Equal(t, "zeta", m.nav.Context)
	assert.NotNil(t, cmd, "switching schedules a reload")
	assert.Empty(t, m.overlayFilter.Value)
	assert.False(t, m.contextPickerFilterMode)
}

func TestContextPicker_EnterAfterFilterSwitchesContext(t *testing.T) {
	m := openPicker(t, contextPickerModel(t))
	m, _ = pressOverlay(t, m, "/", "a", "l", "enter", "enter")

	assert.Equal(t, overlayNone, m.overlay)
	assert.Equal(t, "alpha", m.nav.Context)
	assert.Empty(t, m.overlayFilter.Value)
}

func TestContextPicker_EnterOnCurrentIsNoOp(t *testing.T) {
	m := openPicker(t, contextPickerModel(t))
	before := m.nav
	m, cmd := pressOverlay(t, m, "enter")

	assert.Equal(t, overlayNone, m.overlay)
	assert.Nil(t, cmd)
	assert.Equal(t, before, m.nav)
}

func TestContextPicker_EscAndToggleClose(t *testing.T) {
	m := openPicker(t, contextPickerModel(t))
	m, _ = pressOverlay(t, m, "esc")
	assert.Equal(t, overlayNone, m.overlay)
	assert.Equal(t, "test-ctx", m.nav.Context)

	m = openPicker(t, m)
	m, _ = pressOverlay(t, m, "|")
	assert.Equal(t, overlayNone, m.overlay, "| toggles the picker closed")
}

func TestContextPicker_EscClearsFilterBeforeClosing(t *testing.T) {
	m := openPicker(t, contextPickerModel(t))
	m, _ = pressOverlay(t, m, "/", "z", "enter", "esc")
	assert.Equal(t, overlayContextPicker, m.overlay)
	assert.Empty(t, m.overlayFilter.Value)

	m, _ = pressOverlay(t, m, "esc")
	assert.Equal(t, overlayNone, m.overlay)
}

func TestContextPicker_Navigation(t *testing.T) {
	m := openPicker(t, contextPickerModel(t))
	m, _ = pressOverlay(t, m, "j")
	assert.Equal(t, 2, m.overlayCursor)
	m, _ = pressOverlay(t, m, "j")
	assert.Equal(t, 2, m.overlayCursor, "clamped at the bottom")
	m, _ = pressOverlay(t, m, "g", "g")
	assert.Equal(t, 0, m.overlayCursor)
	m, _ = pressOverlay(t, m, "end")
	assert.Equal(t, 2, m.overlayCursor)
	m, _ = pressOverlay(t, m, "ctrl+u")
	assert.Equal(t, 0, m.overlayCursor)
}

func TestContextPicker_RefusedInUnionMode(t *testing.T) {
	m := contextPickerModel(t)
	m.unionMode = true
	m = openPicker(t, m)

	assert.Equal(t, overlayNone, m.overlay)
	assert.True(t, m.statusMessageErr)
	assert.Contains(t, m.statusMessage, "disabled in union view")
}

func TestContextPicker_RendersListAndHints(t *testing.T) {
	m := openPicker(t, contextPickerModel(t))
	content, _, _, ok := m.renderOverlayContent()
	require.True(t, ok)
	view := stripANSI(content)
	assert.Contains(t, view, "Select Context")
	assert.Contains(t, view, "zeta")
	assert.Contains(t, view, "current")

	assert.Contains(t, stripANSI(m.overlayHintBar()), "switch")
	m, _ = pressOverlay(t, m, "/")
	assert.Contains(t, stripANSI(m.overlayHintBar()), "clear")
}
