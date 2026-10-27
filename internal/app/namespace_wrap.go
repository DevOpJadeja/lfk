package app

import (
	tea "charm.land/bubbletea/v2"
	"github.com/janosmiko/lfk/internal/model"
)

// wrapToFocusedNamespace narrows the namespace scope to the namespace of the
// row under the cursor (k9s "wrap"): on a pod in k8ssandra it lists every pod
// in k8ssandra. The scope it leaves is recorded, so the previous-namespace
// chord returns to it.
func (m Model) wrapToFocusedNamespace() (tea.Model, tea.Cmd) {
	if m.nav.Level == model.LevelClusters {
		m.setStatusMessage("Namespace selection requires a selected context", true)
		return m, scheduleStatusClear()
	}
	if m.unionMode {
		m.setStatusMessage("Union mode supports exactly one namespace", true)
		return m, scheduleStatusClear()
	}
	sel := m.selectedMiddleItem()
	if sel == nil || sel.Namespace == "" {
		m.setStatusMessage("No namespaced resource selected to wrap to", true)
		return m, scheduleStatusClear()
	}
	ns := sel.Namespace
	if m.effectiveNamespace() == ns {
		m.setStatusMessage("Already in namespace: "+ns, false)
		return m, scheduleStatusClear()
	}
	refresh := m.narrowToNamespace(ns, m.captureNamespaceScope())
	m.setStatusMessage("Wrapped to namespace: "+ns, false)
	return m, tea.Batch(refresh, scheduleStatusClear())
}

// wkWrapNamespaceAvailable mirrors wrapToFocusedNamespace's guards.
func wkWrapNamespaceAvailable(c *wkCtx) bool {
	return wkNotAtClusters(c) && !c.m.unionMode && c.sel != nil && c.sel.Namespace != ""
}
