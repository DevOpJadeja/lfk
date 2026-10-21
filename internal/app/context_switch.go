package app

import (
	"fmt"
	"slices"

	tea "charm.land/bubbletea/v2"

	"github.com/janosmiko/lfk/internal/model"
)

// keptContextView is the part of the explorer view a context switch carries
// over to the target context: the resource type the user was on and the row
// to land on inside it.
type keptContextView struct {
	context string
	// typeRef is the ResourceRef of the kept resource type. Empty means
	// there is nothing to keep and the switch lands on the type list.
	typeRef  string
	typeName string
	// descend lands on the resource list of typeRef. When false the cursor
	// only hovers typeRef at the resource-type level.
	descend  bool
	itemName string
	// sortColumn/sortAscending carry the list's sort. Empty column means
	// there is no sort to carry (type list, or deeper than the list).
	sortColumn    string
	sortAscending bool
}

// keptSortCheck marks a carried sort that still has to be checked against
// the rows of the target list: a column the target's rows lack falls back to
// the target's default sort.
type keptSortCheck struct {
	context string
	typeRef string
	column  string
}

// switchContextKeepingView switches the active kube context the same way
// selecting it from the cluster list does, then returns to the resource type
// (and row) the user was looking at and reapplies the namespace scope.
// A resource type missing in the target context falls back to the first row
// of the resource-type list.
func (m Model) switchContextKeepingView(target string) (tea.Model, tea.Cmd) {
	if !slices.Contains(m.contextNames(), target) {
		m.setStatusMessage("Unknown context: "+target, true)
		return m, scheduleStatusClear()
	}
	kept := m.captureKeptView(target)
	scope := m.captureNamespaceScope()
	// Unwinding clears nav.Context, so the cluster-entry path would see an
	// empty old context; remember the scope we are leaving here instead.
	m.rememberScope(m.nav.Context)

	// Unwind to the cluster list the way the LevelCluster key does, so each
	// level's cursor and filter are saved exactly as manual navigation would.
	for m.nav.Level > model.LevelClusters {
		before := m.nav.Level
		ret, _ := m.navigateParent()
		m = ret.(Model)
		if m.nav.Level == before {
			break
		}
	}
	if !m.selectContextRow(target) {
		m.setStatusMessage("Unknown context: "+target, true)
		return m, scheduleStatusClear()
	}

	ret, enterCmd := m.navigateChild()
	m = ret.(Model)
	// With remember_namespaces on, each cluster owns its scope: keep the one
	// navigateChild restored (or the kubeconfig default) instead of carrying.
	if m.rememberEnabled() {
		m.ctxSwitchNsAwaitingCheck = ""
	} else {
		m.applyKeptNamespaceScope(target, scope)
	}
	m.ctxSwitchNsPendingItem = ""
	if m.ctxSwitchNsAwaitingCheck != "" && kept.descend {
		m.ctxSwitchNsPendingItem = kept.itemName
	}

	mdl, landCmd := m.landOrAwaitKeptView(kept)
	return mdl, tea.Batch(enterCmd, landCmd)
}

// ctxSwitchState is the context picker's filter flag plus what a `:ctx`
// switch still has to apply once async data arrives. Embedded in Model, so
// the fields read as m.<field>.
type ctxSwitchState struct {
	contextPickerFilterMode bool // true while typing in the context picker's filter
	// ctxSwitchAwaitingDiscovery holds the view a `:ctx` switch wants to keep
	// when the target context's resource types are not known yet. Consumed by
	// updateAPIResourceDiscovery (see context_switch.go).
	ctxSwitchAwaitingDiscovery *keptContextView
	// ctxSwitchNsAwaitingCheck names the context whose carried-over namespace
	// scope still has to be checked against its namespace list, which had not
	// been fetched yet when the switch happened. Consumed by
	// updateNamespacesLoaded.
	ctxSwitchNsAwaitingCheck string
	// ctxSwitchNsPendingItem is the row to re-select when that deferred
	// check changes the scope and reloads the list.
	ctxSwitchNsPendingItem string
	// ctxSwitchSortCheck is a sort carried by a `:ctx` switch whose column
	// still has to be checked against the target list's rows. Consumed by
	// updateResourcesLoadedMain.
	ctxSwitchSortCheck *keptSortCheck
}

// captureKeptView records what the current view should become in target.
func (m *Model) captureKeptView(target string) keptContextView {
	kept := keptContextView{context: target}
	switch {
	case m.nav.Level == model.LevelResourceTypes:
		if sel := m.selectedMiddleItem(); sel != nil && sel.Extra != "" {
			kept.typeRef = sel.Extra
			kept.typeName = sel.Name
		}
	case m.nav.Level >= model.LevelResources && m.nav.ResourceType.Resource != "":
		kept.typeRef = m.nav.ResourceType.ResourceRef()
		kept.typeName = model.DisplayNameFor(m.nav.ResourceType)
		kept.descend = true
		if m.nav.Level == model.LevelResources {
			if sel := m.selectedMiddleItem(); sel != nil {
				kept.itemName = sel.Name
			}
			// Deeper levels sort a different (child) list, so only the
			// resource list's own sort is carried.
			kept.sortColumn = m.sortColumnName
			kept.sortAscending = m.sortAscending
		} else {
			// Deeper levels (owned resources, containers) return to the
			// parent resource list with the cursor on the parent row.
			kept.itemName = m.nav.ResourceName
		}
	}
	return kept
}

// selectContextRow puts the cursor on target in the cluster list, clearing a
// cluster-list filter that hides it.
func (m *Model) selectContextRow(target string) bool {
	find := func() int {
		for i, item := range m.visibleMiddleItems() {
			if item.Name == target && !isUnionSetItem(&item) {
				return i
			}
		}
		return -1
	}
	idx := find()
	if idx < 0 && m.filterText != "" {
		m.filterText = ""
		m.filterInput.Clear()
		m.filterBroadMode = false
		idx = find()
	}
	if idx < 0 {
		m.setMiddleItems(m.withUnionSetRows(m.contextsOrEmpty()))
		idx = find()
	}
	if idx < 0 {
		return false
	}
	m.setCursor(idx)
	return true
}

// landOrAwaitKeptView applies kept now when the target context's resource
// types can decide it, or parks it until discovery lands.
func (m Model) landOrAwaitKeptView(kept keptContextView) (tea.Model, tea.Cmd) {
	m.ctxSwitchAwaitingDiscovery = nil
	if kept.typeRef == "" {
		return m, nil
	}
	if m.middleHasType(kept.typeRef) || len(m.discoveredResources[kept.context]) > 0 {
		return m.landOnKeptView(kept)
	}
	k := kept
	m.ctxSwitchAwaitingDiscovery = &k
	return m, nil
}

func (m *Model) middleHasType(ref string) bool {
	for _, item := range m.middleItems {
		if item.Extra == ref {
			return true
		}
	}
	return false
}

// landOnKeptView moves from the target context's resource-type list onto the
// kept type. When the type is absent the cursor goes to the first row.
func (m Model) landOnKeptView(kept keptContextView) (tea.Model, tea.Cmd) {
	rt, ok := m.resolveKeptType(kept)
	if !ok || !m.middleHasType(kept.typeRef) {
		m.setCursor(0)
		m.clampCursor()
		m.setStatusMessage(fmt.Sprintf("%s not available in %s", kept.typeName, kept.context), false)
		return m, tea.Batch(m.loadPreview(), scheduleStatusClear())
	}
	if kept.descend {
		m.pendingTargetNamespace = ""
		mdl, cmd := m.teleportToResourceType(rt, kept.itemName)
		landed := mdl.(Model)
		landed.applyKeptSort(kept)
		return landed, cmd
	}
	for _, item := range m.middleItems {
		if item.Extra == kept.typeRef && !m.allGroupsExpanded && item.Category != "" {
			m.expandedGroup = item.Category
			break
		}
	}
	for i, item := range m.visibleMiddleItems() {
		if item.Extra == kept.typeRef {
			m.setCursor(i)
			break
		}
	}
	return m, m.loadPreview()
}

func (m Model) resolveKeptType(kept keptContextView) (model.ResourceTypeEntry, bool) {
	if discovered := m.discoveredResources[kept.context]; len(discovered) > 0 {
		return model.FindResourceTypeIn(kept.typeRef, discovered)
	}
	return model.FindResourceTypeIn(kept.typeRef, append(model.PseudoResources(), model.SeedResources()...))
}

// replayContextSwitchAfterDiscovery applies a parked kept view once the
// target context's resource types arrive (or discovery fails and the seed
// list is all there is). Dropped when the user already moved elsewhere.
func (m Model) replayContextSwitchAfterDiscovery(msgContext string) (Model, tea.Cmd, bool) {
	kept := m.ctxSwitchAwaitingDiscovery
	if kept == nil || kept.context != msgContext {
		return m, nil, false
	}
	m.ctxSwitchAwaitingDiscovery = nil
	if m.nav.Context != msgContext || m.nav.Level != model.LevelResourceTypes || len(m.middleItems) == 0 {
		return m, nil, false
	}
	mdl, cmd := m.landOnKeptView(*kept)
	return mdl.(Model), cmd, true
}

// applyKeptNamespaceScope reapplies the carried namespace scope in target,
// overriding the kubeconfig default navigateChildCluster scoped to. Namespaces
// the target lacks are dropped; when none remain the view shows all
// namespaces. Without a cached namespace list the check waits for the fetch
// navigateChildCluster already fired (ensureNamespaceCacheFresh).
func (m *Model) applyKeptNamespaceScope(target string, scope nsScope) {
	m.ctxSwitchNsAwaitingCheck = ""
	if scope.allNamespaces {
		m.setAllNamespacesScope()
		return
	}
	if scope.namespace == "" && len(scope.selectedNamespaces) == 0 {
		return
	}
	m.applyNamespaceScope(scope)
	if scope.nsSelectionNegated {
		// Excluding a namespace the cluster lacks is harmless.
		return
	}
	if entry, ok := m.cachedNamespaces[target]; ok && len(entry.names) > 0 {
		m.pruneMissingNamespaces(entry.names)
		return
	}
	m.ctxSwitchNsAwaitingCheck = target
}

func (m *Model) setAllNamespacesScope() {
	m.allNamespaces = true
	m.namespace = ""
	m.selectedNamespaces = nil
	m.nsSelectionNegated = false
}

// pruneMissingNamespaces drops selected namespaces absent from existing and
// falls back to all namespaces when nothing is left. Reports whether the
// scope changed.
func (m *Model) pruneMissingNamespaces(existing []string) bool {
	if m.allNamespaces || m.nsSelectionNegated {
		return false
	}
	current := make([]string, 0, len(m.selectedNamespaces)+1)
	for ns, on := range m.selectedNamespaces {
		if on {
			current = append(current, ns)
		}
	}
	if len(current) == 0 && m.namespace != "" {
		current = append(current, m.namespace)
	}
	if len(current) == 0 {
		return false
	}
	kept := make([]string, 0, len(current))
	for _, ns := range current {
		if slices.Contains(existing, ns) {
			kept = append(kept, ns)
		}
	}
	if len(kept) == len(current) {
		return false
	}
	if len(kept) == 0 {
		m.setAllNamespacesScope()
		return true
	}
	slices.Sort(kept)
	m.namespace = kept[0]
	m.selectedNamespaces = make(map[string]bool, len(kept))
	for _, ns := range kept {
		m.selectedNamespaces[ns] = true
	}
	return true
}

// checkNamespacesAfterContextSwitch finishes the deferred namespace check of
// a context switch once the target's namespace list is known, reloading the
// current level when the scope had to change.
func (m Model) checkNamespacesAfterContextSwitch(msg namespacesLoadedMsg) (Model, tea.Cmd) {
	if m.ctxSwitchNsAwaitingCheck == "" || m.ctxSwitchNsAwaitingCheck != msg.context {
		return m, nil
	}
	m.ctxSwitchNsAwaitingCheck = ""
	item := m.ctxSwitchNsPendingItem
	m.ctxSwitchNsPendingItem = ""
	if msg.err != nil || m.nav.Context != msg.context {
		return m, nil
	}
	names := make([]string, 0, len(msg.items))
	for _, item := range msg.items {
		names = append(names, item.Name)
	}
	if !m.pruneMissingNamespaces(names) {
		return m, nil
	}
	if item != "" && m.nav.Level == model.LevelResources {
		// The list loaded under the old scope may have consumed the
		// target already; re-arm it for the reload.
		m.pendingTarget = item
		m.pendingTargetNamespace = ""
	}
	m.saveCurrentSession()
	m.cancelAndReset()
	m.requestGen++
	if m.allNamespaces {
		m.setStatusMessage("Namespace not found in "+msg.context+": showing all namespaces", false)
	} else {
		m.setStatusMessage("Some namespaces not found in "+msg.context, false)
	}
	return m, tea.Batch(m.refreshCurrentLevel(), scheduleStatusClear())
}

// applyKeptSort carries the list sort into the target list without recording
// it in the target's sort memory, then arms the column check. Rows already
// served from cache are checked and re-sorted right away.
func (m *Model) applyKeptSort(kept keptContextView) {
	m.ctxSwitchSortCheck = nil
	if kept.sortColumn == "" || m.nav.Level != model.LevelResources ||
		m.nav.ResourceType.ResourceRef() != kept.typeRef {
		return
	}
	m.sortColumnName = kept.sortColumn
	m.sortAscending = kept.sortAscending
	m.ctxSwitchSortCheck = &keptSortCheck{context: kept.context, typeRef: kept.typeRef, column: kept.sortColumn}
	if len(m.middleItems) > 0 {
		m.checkKeptSort()
		m.sortMiddleItems()
	}
}

// checkKeptSort falls back to the target's default sort (remembered, view
// config, then Name) when none of the loaded rows has the carried column.
// An empty list cannot tell, so the check stays armed for the next load.
func (m *Model) checkKeptSort() {
	c := m.ctxSwitchSortCheck
	if c == nil {
		return
	}
	if m.nav.Context != c.context || m.nav.Level != model.LevelResources ||
		m.nav.ResourceType.ResourceRef() != c.typeRef {
		m.ctxSwitchSortCheck = nil
		return
	}
	if len(m.middleItems) == 0 {
		return
	}
	m.ctxSwitchSortCheck = nil
	if m.sortColumnName != c.column {
		return // the user picked another sort meanwhile
	}
	if !sortColumnPresent(m.middleItems, c.column) {
		m.applyResourceTypeSortDefault(m.nav.ResourceType, m.nav.Context)
	}
}

// sortColumnPresent reports whether any row carries a value for the sort
// column col, using the same fields comparePrimaryColumn sorts on.
func sortColumnPresent(items []model.Item, col string) bool {
	for _, it := range items {
		var present bool
		switch col {
		case "Name":
			return true
		case "Namespace":
			present = it.Namespace != ""
		case "Context":
			present = it.ClusterName != ""
		case "Ready":
			present = it.Ready != ""
		case "Restarts":
			present = it.Restarts != ""
		case "Status":
			present = it.Status != ""
		case "Age":
			present = it.Age != "" || !it.CreatedAt.IsZero()
		default:
			present = slices.ContainsFunc(it.Columns, func(kv model.KeyValue) bool { return kv.Key == col })
		}
		if present {
			return true
		}
	}
	return false
}
