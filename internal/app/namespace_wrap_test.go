package app

import (
	"testing"

	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// wrapTestModel is an all-namespaces pod list with the cursor on a pod in
// k8ssandra, the scenario wrap exists for.
func wrapTestModel() Model {
	m := basePush80Model()
	m.allNamespaces = true
	m.namespace = ""
	m.middleItems = []model.Item{
		{Name: "web-0", Kind: "Pod", Namespace: "default"},
		{Name: "cassandra-0", Kind: "Pod", Namespace: "k8ssandra"},
	}
	m.setCursor(1)
	return m
}

func TestWrapNamespaceDefaultKey(t *testing.T) {
	kb := ui.DefaultKeybindings()
	assert.Equal(t, "w", kb.WrapNamespace)
	assert.Empty(t, kb.WatchMode, "watch mode gives up w to wrap and ships unbound")
}

func TestWrapNamespace_NarrowsToFocusedItemNamespace(t *testing.T) {
	m := wrapTestModel()

	out, cmd := m.handleKey(keyMsg(ui.ActiveKeybindings.WrapNamespace))
	got := out.(Model)

	assert.NotNil(t, cmd, "wrap reloads the current level")
	assert.False(t, got.allNamespaces)
	assert.Equal(t, "k8ssandra", got.namespace)
	assert.Equal(t, map[string]bool{"k8ssandra": true}, got.selectedNamespaces)
	assert.Equal(t, "k8ssandra", got.effectiveNamespace())
	if assert.NotNil(t, got.previousNsScope, "wrap records the scope it left") {
		assert.True(t, got.previousNsScope.allNamespaces)
	}
}

func TestWrapNamespace_PreviousNamespaceRestoresAllNamespaces(t *testing.T) {
	m := wrapTestModel()

	out, _ := m.wrapToFocusedNamespace()
	out, _ = out.(Model).jumpToPreviousNamespace()
	got := out.(Model)

	assert.True(t, got.allNamespaces, "g\\ returns to the pre-wrap scope")
	assert.Empty(t, got.selectedNamespaces)
}

func TestWrapNamespace_AlreadyInNamespaceIsNoop(t *testing.T) {
	tests := map[string]map[string]bool{
		"selected set":    {"k8ssandra": true},
		"no selected set": nil, // e.g. after toggling all-namespaces off
	}
	for name, selected := range tests {
		t.Run(name, func(t *testing.T) {
			m := wrapTestModel()
			m.allNamespaces = false
			m.namespace = "k8ssandra"
			m.selectedNamespaces = selected
			gen := m.requestGen

			out, _ := m.wrapToFocusedNamespace()
			got := out.(Model)

			assert.Equal(t, gen, got.requestGen, "no reload when the scope is unchanged")
			assert.Nil(t, got.previousNsScope)
			assert.False(t, got.statusMessageErr)
			assert.Contains(t, got.statusMessage, "k8ssandra")
		})
	}
}

func TestWrapNamespace_Rejected(t *testing.T) {
	tests := []struct {
		name  string
		setup func(*Model)
	}{
		{"cluster-scoped row", func(m *Model) {
			m.middleItems = []model.Item{{Name: "node-1", Kind: "Node"}}
			m.setCursor(0)
		}},
		{"empty list", func(m *Model) { m.middleItems = nil }},
		{"clusters level", func(m *Model) { m.nav.Level = model.LevelClusters }},
		{"union mode", func(m *Model) { m.unionMode = true }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := wrapTestModel()
			tt.setup(&m)
			before := m.captureNamespaceScope()

			out, _ := m.wrapToFocusedNamespace()
			got := out.(Model)

			assert.True(t, got.statusMessageErr, "rejection surfaces an error status")
			require.NotEmpty(t, got.statusMessage)
			assert.True(t, before.equal(got.captureNamespaceScope()), "scope is left unchanged")
		})
	}
}

func TestWrapNamespace_WhichKeyOffer(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()

	m := wrapTestModel()
	assert.Contains(t, whichKeyLabels(m), "Wrap to row namespace")

	m.middleItems = []model.Item{{Name: "node-1", Kind: "Node"}}
	m.setCursor(0)
	assert.NotContains(t, whichKeyLabels(m), "Wrap to row namespace", "cluster-scoped row has nothing to wrap to")
}

// bindWatchModeForTest binds the (default-unbound) watch mode action for one
// test and returns the active keybindings.
func bindWatchModeForTest(t *testing.T) ui.Keybindings {
	t.Helper()
	prev := ui.ActiveKeybindings
	t.Cleanup(func() { ui.ActiveKeybindings = prev })
	ui.ActiveKeybindings.WatchMode = "V"
	return ui.ActiveKeybindings
}
