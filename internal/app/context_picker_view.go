package app

import (
	"github.com/janosmiko/lfk/internal/ui"
)

// overlayContextPickerScrollPos keeps the picker's scrolloff viewport sticky
// across renders (see the per-overlay scroll vars in view_overlay_lists.go).
var overlayContextPickerScrollPos int

// renderOverlayContextPicker renders the context picker as a standard
// centered list overlay. The "current" marker sits in the description column.
func (m Model) renderOverlayContextPicker() (string, int, int) {
	w, h := min(60, m.width-10), min(20, m.height-6)
	src := m.filteredOverlayItems()
	items := make([]ui.OverlayListItem, len(src))
	for i, it := range src {
		items[i] = ui.OverlayListItem{Name: it.Name, Description: it.Status}
	}
	const maxVisible = 15
	content := ui.RenderOverlayList(items, ui.OverlayListConfig{
		Title:           "Select Context",
		Cursor:          m.overlayCursor,
		Filterable:      true,
		Filter:          m.overlayFilter.Value,
		FilterActive:    m.contextPickerFilterMode,
		FilterModeAware: true,
		ShowDescription: true,
		Scroll:          overlayListScroll(&overlayContextPickerScrollPos, m.overlayCursor, len(src), maxVisible),
		MaxVisible:      maxVisible,
		EmptyMessage:    "No matching contexts",
	}, w-4)
	return content, w, h
}

// overlayHintBarContextPicker returns the hint bar for the context picker,
// switching to the filter-input variant while "/" filtering is active.
func (m Model) overlayHintBarContextPicker() string {
	if m.contextPickerFilterMode {
		return m.renderHints([]ui.HintEntry{
			{Key: "type", Desc: "filter"},
			ui.SearchModeHintEntry(),
			{Key: "enter", Desc: "apply"},
			{Key: "esc", Desc: "clear"},
		})
	}
	return m.renderHints([]ui.HintEntry{
		{Key: "j/k", Desc: "navigate"},
		{Key: "g/G", Desc: "top/bottom"},
		{Key: "enter", Desc: "switch"},
		{Key: "/", Desc: "filter"},
		{Key: "esc", Desc: "close"},
	})
}
