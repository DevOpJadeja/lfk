package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

// rememberCtxKeepViewModel is ctxKeepViewModel scoped to team-a with
// remember_namespaces turned on.
func rememberCtxKeepViewModel(t *testing.T) Model {
	t.Helper()
	t.Setenv("LFK_STATE_DIR", t.TempDir())
	orig := ui.ConfigRememberNamespaces
	t.Cleanup(func() { ui.ConfigRememberNamespaces = orig })
	ui.ConfigRememberNamespaces = true
	m := withTeamANamespace(ctxKeepViewModel(t))
	cacheQANamespaces(&m, "default", "team-a")
	return m
}

func TestContextSwitch_RememberModeUsesTargetScopeAndKeepsView(t *testing.T) {
	m := rememberCtxKeepViewModel(t)

	ret, _ := m.executeBuiltinCommand("ctx qa")
	got := ret.(Model)

	assert.Equal(t, "default", got.namespace, "qa's kubeconfig namespace, not the carried team-a")
	assert.Empty(t, got.ctxSwitchNsAwaitingCheck)
	assert.Equal(t, model.LevelResources, got.nav.Level)
	assert.Equal(t, podsRef, got.nav.ResourceType.ResourceRef())
	assert.Equal(t, "api-1", got.pendingTarget)

	left, ok := loadRememberedScope("test-ctx")
	require.True(t, ok, "the scope being left must be remembered")
	assert.Equal(t, "team-a", left.namespace)
}

func TestContextSwitch_RememberModeRestoresRememberedScope(t *testing.T) {
	m := rememberCtxKeepViewModel(t)
	saveRememberedScopes(map[string]nsScope{"qa": {namespace: "kept-in-qa"}})

	ret, _ := m.executeBuiltinCommand("ctx qa")
	got := ret.(Model)

	assert.Equal(t, "kept-in-qa", got.namespace)
	assert.Equal(t, podsRef, got.nav.ResourceType.ResourceRef())
}
