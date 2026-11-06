package components

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"mygo_demo/internal/api"
	"mygo_demo/internal/database/repo"
)

// nodeLayoutVersion marks the JSON shape of nodeLayout, so a future change
// can migrate or drop an old value.
const nodeLayoutVersion = 1

// nodeLayout is the saved arrangement of the node list: the pinned nodes in
// their own order, and the custom order of the rest.
type nodeLayout struct {
	Version int      `json:"version"`
	Pins    []string `json:"pins"`
	Order   []string `json:"order"`
}

// nodeRows is what the node list shows: pinned nodes on top, the rest below.
type nodeRows struct {
	pinned []api.AllNodeInfo
	normal []api.AllNodeInfo
}

// loadNodeLayout reads the saved arrangement. A missing or broken value
// leaves an empty layout, as a first start; the node list then follows the
// hot order.
func loadNodeLayout(settings *repo.Settings) nodeLayout {
	raw, ok, err := settings.Get(repo.NodeLayoutKey)
	if err != nil || !ok || raw == "" {
		return nodeLayout{Version: nodeLayoutVersion}
	}

	var layout nodeLayout
	if err := json.Unmarshal([]byte(raw), &layout); err != nil {
		return nodeLayout{Version: nodeLayoutVersion}
	}
	return layout
}

// saveNodeLayout stores layout, stamping the current version.
func saveNodeLayout(settings *repo.Settings, layout nodeLayout) error {
	layout.Version = nodeLayoutVersion
	raw, err := json.Marshal(layout)
	if err != nil {
		return fmt.Errorf("encode node layout: %w", err)
	}
	if err := settings.Set(repo.NodeLayoutKey, string(raw)); err != nil {
		return fmt.Errorf("save node layout: %w", err)
	}
	return nil
}

// move applies a ListState.Reorder move to names: the names at rows go
// before row to, or at the end when to is the number of rows.
func move(names []string, rows []int, to int) []string {
	moved := make([]string, 0, len(rows))
	rest := make([]string, 0, len(names))
	at := -1
	for i, name := range names {
		if i == to {
			at = len(rest)
		}
		if slices.Contains(rows, i) {
			moved = append(moved, name)
		} else {
			rest = append(rest, name)
		}
	}
	if at < 0 {
		at = len(rest)
	}
	return slices.Insert(rest, at, moved...)
}

// nodeRowsFor builds both sections for the current view. The pinned section
// keeps its own order everywhere; the custom order applies to the normal
// section only in the full view with no search, while the favorites list and
// search results keep the hot order.
func (v *View) nodeRowsFor() nodeRows {
	q := strings.ToLower(strings.TrimSpace(v.Query))

	starred := map[string]bool{}
	if favorites, err := v.FavoritesRepo.Favorites(); err == nil {
		for _, f := range favorites {
			starred[f.NodeName] = true
		}
	}

	nodes := v.everyNode()
	byName := make(map[string]api.AllNodeInfo, len(nodes))
	for _, n := range nodes {
		byName[n.Name] = n
	}

	pinned := make(map[string]bool, len(v.nodeLayout.Pins))
	for _, name := range v.nodeLayout.Pins {
		pinned[name] = true
	}

	matches := func(n api.AllNodeInfo) bool {
		if v.Filter == 1 && !starred[n.Name] {
			return false
		}
		if q == "" {
			return true
		}
		haystack := strings.ToLower(n.Title + " " + n.Name + " " + n.TitleAlternative)
		return strings.Contains(haystack, q)
	}
	add := func(dst *[]api.AllNodeInfo, n api.AllNodeInfo) {
		if !matches(n) {
			return
		}
		n.IsFavorite = starred[n.Name]
		*dst = append(*dst, n)
	}

	rows := nodeRows{}

	// Pinned rows come first, in their own order, and only when they pass
	// the current filter or search.
	for _, name := range v.nodeLayout.Pins {
		if n, ok := byName[name]; ok {
			add(&rows.pinned, n)
		}
	}

	if v.Filter == 0 && q == "" && len(v.nodeLayout.Order) > 0 {
		seen := map[string]bool{}
		for _, name := range v.nodeLayout.Order {
			n, ok := byName[name]
			if !ok || pinned[name] {
				continue
			}
			seen[name] = true
			add(&rows.normal, n)
		}
		// Nodes added since the order was saved follow the hot order.
		for _, n := range nodes {
			if seen[n.Name] || pinned[n.Name] {
				continue
			}
			add(&rows.normal, n)
		}
		return rows
	}

	for _, n := range nodes {
		if pinned[n.Name] {
			continue
		}
		add(&rows.normal, n)
	}
	return rows
}

// isPinned reports whether the node is in the pinned section.
func (v *View) isPinned(name string) bool {
	return slices.Contains(v.nodeLayout.Pins, name)
}

// togglePin adds the node to the pinned section, or takes it out. A new pin
// goes to the end, so several pins keep the order they were pinned in.
func (v *View) togglePin(name string) {
	pins := make([]string, 0, len(v.nodeLayout.Pins)+1)
	removed := false
	for _, p := range v.nodeLayout.Pins {
		if p == name {
			removed = true
			continue
		}
		pins = append(pins, p)
	}
	if !removed {
		pins = append(pins, name)
	}

	v.nodeLayout.Pins = pins
	v.persistNodeLayout()
	v.invalidate()
}

// reorderPinned applies a drag within the pinned section; the shown names
// become the pin order.
func (v *View) reorderPinned(shown []api.AllNodeInfo, rows []int, to int) {
	names := make([]string, len(shown))
	for i, n := range shown {
		names[i] = n.Name
	}
	v.nodeLayout.Pins = move(names, rows, to)
	v.persistNodeLayout()
	v.invalidate()
}

// reorderNormal applies a drag within the normal section; the shown names
// become the custom order, replacing names that are gone.
func (v *View) reorderNormal(shown []api.AllNodeInfo, rows []int, to int) {
	names := make([]string, len(shown))
	for i, n := range shown {
		names[i] = n.Name
	}
	v.nodeLayout.Order = move(names, rows, to)
	v.persistNodeLayout()
	v.invalidate()
}

// persistNodeLayout saves the arrangement; a failure keeps what the view
// shows and tells the user it will be lost.
func (v *View) persistNodeLayout() {
	if err := saveNodeLayout(v.SettingsRepo, v.nodeLayout); err != nil {
		v.ShowToast("保存节点排序失败")
	}
}
