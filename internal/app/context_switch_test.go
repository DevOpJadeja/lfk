package app

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/model"
)

const podsRef = "/v1/pods"

var argoAppsEntry = model.ResourceTypeEntry{
	Kind: "Application", APIGroup: "argoproj.io", APIVersion: "v1alpha1",
	Resource: "applications", Namespaced: true,
}

func ctxSwitchEntries(extra ...model.ResourceTypeEntry) []model.ResourceTypeEntry {
	entries := append(model.PseudoResources(), model.SeedResources()...)
	return append(entries, extra...)
}

// ctxKeepViewModel opens test-ctx from the cluster list and lands on the Pods
// list with the cursor on api-1, using the same navigation a user would.
func ctxKeepViewModel(t *testing.T, devExtra ...model.ResourceTypeEntry) Model {
	t.Helper()
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	m := baseModelWithFakeClient()
	m.client.AddTestContextWithNamespace("qa", "https://qa.example.local:6443", "default")
	m.discoveredResources["test-ctx"] = ctxSwitchEntries(devExtra...)
	m.discoveredResources["qa"] = ctxSwitchEntries()
	m.discoveryRefreshedContexts["test-ctx"] = true
	m.discoveryRefreshedContexts["qa"] = true

	m.nav = model.NavigationState{Level: model.LevelClusters}
	m.setMiddleItems(m.contextsOrEmpty())
	require.True(t, m.selectContextRow("test-ctx"))
	ret, _ := m.navigateChild()
	m = ret.(Model)
	require.Equal(t, model.LevelResourceTypes, m.nav.Level)

	rt, ok := model.FindResourceTypeIn(podsRef, m.discoveredResources["test-ctx"])
	require.True(t, ok)
	ret, _ = m.teleportToResourceType(rt, "")
	m = ret.(Model)
	require.Equal(t, model.LevelResources, m.nav.Level)
	m.setMiddleItems([]model.Item{
		{Name: "api-0", Namespace: "default", Kind: "Pod"},
		{Name: "api-1", Namespace: "default", Kind: "Pod"},
	})
	m.setCursor(1)
	m.pendingTarget = ""
	return m
}

func TestContextSwitch_KeepsResourceTypeAndRow(t *testing.T) {
	m := ctxKeepViewModel(t)

	ret, _ := m.executeBuiltinCommand("ctx qa")
	got := ret.(Model)

	assert.Equal(t, "qa", got.nav.Context)
	assert.Equal(t, model.LevelResources, got.nav.Level, "must stay on the resource list, not the type list")
	assert.Equal(t, podsRef, got.nav.ResourceType.ResourceRef())
	assert.Equal(t, "api-1", got.pendingTarget, "cursor must land on the same-named row once loaded")
}

func qaPods() []model.Item {
	return []model.Item{
		{Name: "api-0", Namespace: "default", Kind: "Pod"},
		{Name: "api-1", Namespace: "default", Kind: "Pod"},
		{Name: "api-2", Namespace: "default", Kind: "Pod"},
	}
}

func TestContextSwitch_LoadedListShowsSameRow(t *testing.T) {
	m := ctxKeepViewModel(t)

	ret, _ := m.executeBuiltinCommand("ctx qa")
	got := ret.(Model)
	mdl, _ := got.updateResourcesLoaded(resourcesLoadedMsg{gen: got.requestGen, context: "qa", items: qaPods()})
	got = mdl.(Model)

	assert.Equal(t, model.LevelResources, got.nav.Level)
	assert.Equal(t, "Pod", got.middleItems[0].Kind, "middle pane must hold the qa pods, not resource types")
	if sel := got.selectedMiddleItem(); assert.NotNil(t, sel) {
		assert.Equal(t, "api-1", sel.Name)
	}
}

func TestContextSwitch_ContextAliasBehavesTheSame(t *testing.T) {
	m := ctxKeepViewModel(t)

	ret, _ := m.executeBuiltinCommand("context qa")
	got := ret.(Model)

	assert.Equal(t, "qa", got.nav.Context)
	assert.Equal(t, model.LevelResources, got.nav.Level)
	assert.Equal(t, podsRef, got.nav.ResourceType.ResourceRef())
}

func TestContextSwitch_DeeperLevelKeepsParentRow(t *testing.T) {
	m := ctxKeepViewModel(t)
	m.nav.Level = model.LevelContainers
	m.nav.ResourceName = "api-1"

	kept := m.captureKeptView("qa")

	assert.True(t, kept.descend)
	assert.Equal(t, podsRef, kept.typeRef)
	assert.Equal(t, "api-1", kept.itemName, "containers view must return to the parent pod row")
}

func TestContextSwitch_MissingTypeFallsBackToFirstRow(t *testing.T) {
	m := ctxKeepViewModel(t, argoAppsEntry)
	ret, _ := m.teleportToResourceType(argoAppsEntry, "")
	m = ret.(Model)
	require.Equal(t, "applications", m.nav.ResourceType.Resource)

	ret, _ = m.executeBuiltinCommand("ctx qa")
	got := ret.(Model)

	assert.Equal(t, "qa", got.nav.Context)
	assert.Equal(t, model.LevelResourceTypes, got.nav.Level)
	assert.Equal(t, 0, got.cursor(), "missing type must fall back to the first row")
	if sel := got.selectedMiddleItem(); assert.NotNil(t, sel) {
		assert.Equal(t, "Cluster", sel.Name)
	}
	assert.False(t, got.statusMessageErr, "the fallback is not an error")
}

func TestContextSwitch_WaitsForDiscoveryThenLands(t *testing.T) {
	m := ctxKeepViewModel(t)
	delete(m.discoveredResources, "qa")
	delete(m.discoveryRefreshedContexts, "qa")

	ret, _ := m.executeBuiltinCommand("ctx qa")
	got := ret.(Model)
	require.Equal(t, model.LevelResourceTypes, got.nav.Level)
	require.NotNil(t, got.ctxSwitchAwaitingDiscovery)

	got, _ = got.updateAPIResourceDiscovery(apiResourceDiscoveryMsg{
		context: "qa",
		entries: model.SeedResources(),
	})

	assert.Nil(t, got.ctxSwitchAwaitingDiscovery)
	assert.Equal(t, model.LevelResources, got.nav.Level)
	assert.Equal(t, podsRef, got.nav.ResourceType.ResourceRef())
	assert.Equal(t, "api-1", got.pendingTarget)
}

func TestContextSwitch_ResourceTypeLevelKeepsHoveredType(t *testing.T) {
	m := ctxKeepViewModel(t)
	for m.nav.Level > model.LevelResourceTypes {
		ret, _ := m.navigateParent()
		m = ret.(Model)
	}
	sel := m.selectedMiddleItem()
	require.NotNil(t, sel)
	require.Equal(t, podsRef, sel.Extra)

	ret, _ := m.executeBuiltinCommand("ctx qa")
	got := ret.(Model)

	assert.Equal(t, model.LevelResourceTypes, got.nav.Level)
	if s := got.selectedMiddleItem(); assert.NotNil(t, s) {
		assert.Equal(t, podsRef, s.Extra)
	}
}

func TestContextSwitch_UnknownContextIsRejected(t *testing.T) {
	m := ctxKeepViewModel(t)

	ret, _ := m.executeBuiltinCommand("ctx nope")
	got := ret.(Model)

	assert.Equal(t, "test-ctx", got.nav.Context)
	assert.Equal(t, model.LevelResources, got.nav.Level)
	assert.True(t, got.statusMessageErr)
}

func TestContextSwitch_UnionModeIsRejected(t *testing.T) {
	m := ctxKeepViewModel(t)
	m.unionMode = true

	ret, _ := m.executeBuiltinCommand("ctx qa")
	got := ret.(Model)

	assert.Equal(t, "test-ctx", got.nav.Context)
	assert.True(t, got.statusMessageErr)
}

// withTeamANamespace scopes m to the single namespace team-a.
func withTeamANamespace(m Model) Model {
	const ns = "team-a"
	m.allNamespaces = false
	m.namespace = ns
	m.selectedNamespaces = map[string]bool{ns: true}
	m.nsSelectionNegated = false
	return m
}

func cacheQANamespaces(m *Model, names ...string) {
	if m.cachedNamespaces == nil {
		m.cachedNamespaces = make(map[string]namespaceCacheEntry)
	}
	m.cachedNamespaces["qa"] = namespaceCacheEntry{names: names, fetchedAt: time.Now()}
}

func TestContextSwitch_KeepsNamespacePresentInTarget(t *testing.T) {
	m := withTeamANamespace(ctxKeepViewModel(t))
	cacheQANamespaces(&m, "default", "team-a")

	ret, _ := m.executeBuiltinCommand("ctx qa")
	got := ret.(Model)

	assert.False(t, got.allNamespaces)
	assert.Equal(t, "team-a", got.namespace)
	assert.Equal(t, map[string]bool{"team-a": true}, got.selectedNamespaces)
}

func TestContextSwitch_MissingNamespaceShowsAll(t *testing.T) {
	m := withTeamANamespace(ctxKeepViewModel(t))
	cacheQANamespaces(&m, "default")

	ret, _ := m.executeBuiltinCommand("ctx qa")
	got := ret.(Model)

	assert.True(t, got.allNamespaces)
	assert.Empty(t, got.selectedNamespaces)
}

func TestContextSwitch_PartialNamespacesArePruned(t *testing.T) {
	m := ctxKeepViewModel(t)
	m.allNamespaces = false
	m.namespace = "team-a"
	m.selectedNamespaces = map[string]bool{"team-a": true, "team-b": true}
	cacheQANamespaces(&m, "team-b")

	ret, _ := m.executeBuiltinCommand("ctx qa")
	got := ret.(Model)

	assert.False(t, got.allNamespaces)
	assert.Equal(t, "team-b", got.namespace)
	assert.Equal(t, map[string]bool{"team-b": true}, got.selectedNamespaces)
}

func TestContextSwitch_AllNamespacesCarriesOver(t *testing.T) {
	m := ctxKeepViewModel(t)
	m.allNamespaces = true
	m.selectedNamespaces = nil

	ret, _ := m.executeBuiltinCommand("ctx qa")
	got := ret.(Model)

	assert.True(t, got.allNamespaces)
}

func TestContextSwitch_NamespaceCheckWaitsForFetch(t *testing.T) {
	m := withTeamANamespace(ctxKeepViewModel(t))

	ret, _ := m.executeBuiltinCommand("ctx qa")
	got := ret.(Model)
	require.Equal(t, "qa", got.ctxSwitchNsAwaitingCheck)
	require.Equal(t, "team-a", got.namespace, "scope is applied provisionally until the list arrives")

	mdl, cmd := got.updateNamespacesLoaded(namespacesLoadedMsg{
		context: "qa",
		items:   []model.Item{{Name: "default"}},
		silent:  true,
	})
	got = mdl.(Model)

	assert.Empty(t, got.ctxSwitchNsAwaitingCheck)
	assert.True(t, got.allNamespaces, "missing namespace must fall back to all namespaces")
	assert.NotNil(t, cmd, "the current level must reload under the new scope")
	assert.Equal(t, "api-1", got.pendingTarget, "the reload must land on the kept row again")
}

func TestContextSwitch_NamespaceFallbackKeepsRowWhenListArrivedFirst(t *testing.T) {
	m := withTeamANamespace(ctxKeepViewModel(t))

	ret, _ := m.executeBuiltinCommand("ctx qa")
	got := ret.(Model)
	// The list for the provisional (missing) namespace lands first, empty.
	mdl, _ := got.updateResourcesLoaded(resourcesLoadedMsg{gen: got.requestGen, context: "qa"})
	got = mdl.(Model)
	require.Empty(t, got.pendingTarget)

	mdl, _ = got.updateNamespacesLoaded(namespacesLoadedMsg{
		context: "qa", items: []model.Item{{Name: "default"}}, silent: true,
	})
	got = mdl.(Model)
	require.True(t, got.allNamespaces)
	mdl, _ = got.updateResourcesLoaded(resourcesLoadedMsg{gen: got.requestGen, context: "qa", items: qaPods()})
	got = mdl.(Model)

	if sel := got.selectedMiddleItem(); assert.NotNil(t, sel) {
		assert.Equal(t, "api-1", sel.Name)
	}
}

func TestContextSwitch_NamespaceCheckKeepsExistingAfterFetch(t *testing.T) {
	m := withTeamANamespace(ctxKeepViewModel(t))

	ret, _ := m.executeBuiltinCommand("ctx qa")
	got := ret.(Model)

	mdl, _ := got.updateNamespacesLoaded(namespacesLoadedMsg{
		context: "qa",
		items:   []model.Item{{Name: "team-a"}},
		silent:  true,
	})
	got = mdl.(Model)

	assert.Empty(t, got.ctxSwitchNsAwaitingCheck)
	assert.False(t, got.allNamespaces)
	assert.Equal(t, "team-a", got.namespace)
}

func sortedQAPods() []model.Item {
	return []model.Item{
		{Name: "api-0", Namespace: "default", Kind: "Pod", Age: "3d", CreatedAt: time.Now().Add(-72 * time.Hour)},
		{Name: "api-1", Namespace: "default", Kind: "Pod", Age: "1h", CreatedAt: time.Now().Add(-time.Hour)},
		{Name: "api-2", Namespace: "default", Kind: "Pod", Age: "2d", CreatedAt: time.Now().Add(-48 * time.Hour)},
	}
}

// withDescSort sorts m's list by col, descending.
func withDescSort(m Model, col string) Model {
	m.sortColumnName = col
	m.sortAscending = false
	return m
}

func TestContextSwitch_KeepsSortColumn(t *testing.T) {
	m := withDescSort(ctxKeepViewModel(t), "Age")

	ret, _ := m.executeBuiltinCommand("ctx qa")
	got := ret.(Model)
	require.Equal(t, "Age", got.sortColumnName)
	require.False(t, got.sortAscending)

	mdl, _ := got.updateResourcesLoaded(resourcesLoadedMsg{gen: got.requestGen, context: "qa", items: sortedQAPods()})
	got = mdl.(Model)

	assert.Equal(t, "Age", got.sortColumnName, "a column present in the target keeps the sort")
	assert.False(t, got.sortAscending)
	assert.Nil(t, got.ctxSwitchSortCheck)
	names := []string{got.middleItems[0].Name, got.middleItems[1].Name, got.middleItems[2].Name}
	assert.Equal(t, []string{"api-0", "api-2", "api-1"}, names, "Age descending puts the oldest row first")
	assert.Empty(t, got.sortMemory, "the carried sort must not be saved as the target's preference")
}

func TestContextSwitch_MissingSortColumnFallsBackToDefault(t *testing.T) {
	m := withDescSort(ctxKeepViewModel(t), "Hostnames")

	ret, _ := m.executeBuiltinCommand("ctx qa")
	got := ret.(Model)
	mdl, _ := got.updateResourcesLoaded(resourcesLoadedMsg{gen: got.requestGen, context: "qa", items: sortedQAPods()})
	got = mdl.(Model)

	assert.Equal(t, sortColDefault, got.sortColumnName)
	assert.True(t, got.sortAscending)
	assert.Nil(t, got.ctxSwitchSortCheck)
}

func TestContextSwitch_SortCheckWaitsForRows(t *testing.T) {
	m := withDescSort(ctxKeepViewModel(t), "Hostnames")

	ret, _ := m.executeBuiltinCommand("ctx qa")
	got := ret.(Model)
	mdl, _ := got.updateResourcesLoaded(resourcesLoadedMsg{gen: got.requestGen, context: "qa"})
	got = mdl.(Model)
	require.NotNil(t, got.ctxSwitchSortCheck, "an empty list cannot decide; keep the check armed")
	require.Equal(t, "Hostnames", got.sortColumnName)

	mdl, _ = got.updateResourcesLoaded(resourcesLoadedMsg{gen: got.requestGen, context: "qa", items: sortedQAPods()})
	got = mdl.(Model)

	assert.Equal(t, sortColDefault, got.sortColumnName)
}

func TestContextSwitch_DeeperLevelDoesNotCarrySort(t *testing.T) {
	m := withDescSort(ctxKeepViewModel(t), "Age")
	m.nav.Level = model.LevelContainers
	m.nav.ResourceName = "api-1"

	kept := m.captureKeptView("qa")

	assert.Empty(t, kept.sortColumn, "a child list's sort does not apply to the parent list")
}

func TestSortColumnPresent(t *testing.T) {
	items := []model.Item{
		{Name: "a", Ready: "1/1", Columns: []model.KeyValue{{Key: "Hostnames", Value: "x.example"}}},
		{Name: "b"},
	}
	assert.True(t, sortColumnPresent(items, "Name"))
	assert.True(t, sortColumnPresent(items, "Ready"))
	assert.True(t, sortColumnPresent(items, "Hostnames"))
	assert.False(t, sortColumnPresent(items, "Namespace"))
	assert.False(t, sortColumnPresent(items, "Parents"))
	assert.False(t, sortColumnPresent(nil, "Name"))
}
