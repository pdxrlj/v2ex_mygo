package components

import (
	"fmt"
	"strings"

	"mygo_demo/internal/api"
	"mygo_demo/internal/ui/components/icons"

	"github.com/egoist/mygo/ui"
)

// Node row metrics: rows have a fixed height so the virtual list measures
// them once, and the pinned section can size itself without a frame of jitter.
const (
	nodeRowHeight float32 = 56
	nodeRowGap    float32 = 6
	pinnedMaxRows         = 4
)

// pinnedSectionHeight is the height of the pinned list for n rows: up to
// pinnedMaxRows of them, the rest scrolling inside it.
func pinnedSectionHeight(n int) float32 {
	shown := min(n, pinnedMaxRows)
	if shown <= 0 {
		return 0
	}
	return float32(shown)*nodeRowHeight + float32(shown-1)*nodeRowGap
}

// NodeList draws the middle column: its search and filters, then the pinned
// nodes and the rest, each list reorderable by dragging in the full view.
func (v *View) NodeList(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Width(270).FillHeight().
		Background(t.Surface).
		BorderWidth(0, 1, 0, 0).
		BorderLinearGradient(fadeDivider(t)).
		Children(func() {
			ui.Column(c).Cursor(ui.CursorPointer).FillWidth().Padding(14, 12).Gap(10).
				Children(func() {
					if ui.SearchField(c, &v.Query).
						Placeholder("搜索节点（名称/标题/别名）…").
						FontSize(14).
						Height(36).
						Padding(6, 10).
						Label("搜索节点").
						Changed() {
						v.invalidate()
					}

					ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(8).
						Children(func() {
							v.filterButton(c,
								"全部 ("+formatNumber(v.nodeTotal())+")", 0)
							v.filterButton(c,
								"已收藏 "+formatNumber(v.favoritesCount()), 1)
							ui.Spacer(c)
							ui.ButtonBase(c).Label("排序").
								Padding(4, 8).
								Radius(6).
								Border(1, t.Border.Alpha(0.4)).
								Children(func() {
									ui.Text(c, "主题数").FontSize(11).TextColor(t.TextMuted)
									ui.Text(c, " ▾").FontSize(10).TextColor(t.TextMuted)
								}).
								OnClick(func() {
									c.Toast("排序方式：主题数从多到少")
								})
						})
				})

			ui.Box(c).FillWidth().Height(1).Background(t.Border.Alpha(0.25))

			rows := v.nodeRowsFor()
			// Dragging changes the saved arrangement, which the search and
			// favorites views do not show.
			draggable := v.Filter == 0 && strings.TrimSpace(v.Query) == ""

			if len(rows.pinned)+len(rows.normal) == 0 {
				ui.Column(c).FillWidth().Grow(1).Padding(24).Center().
					Children(func() {
						ui.Icon(c, icons.Nodes).FontSize(28).TextColor(t.TextMuted)
						ui.Text(c, "没有匹配的节点").FontSize(12).TextColor(t.TextMuted)
					})
				return
			}

			if len(rows.pinned) > 0 {
				ui.Text(c, "置顶").FontSize(11).FontWeight(600).
					TextColor(t.TextMuted).Padding(10, 12, 4, 12)

				v.pinListState(rows.pinned, draggable)
				ui.List(c, &v.PinListState, len(rows.pinned), func(i int) {
					v.nodeRow(c, rows.pinned[i])
				}).FillWidth().
					Height(pinnedSectionHeight(len(rows.pinned))).
					Padding(0, 12).
					Gap(nodeRowGap)
			}

			if len(rows.normal) > 0 {
				v.nodeListState(rows.normal, draggable)
				ui.List(c, &v.NormalListState, len(rows.normal), func(i int) {
					v.nodeRow(c, rows.normal[i])
				}).FillWidth().Grow(1).Padding(10, 12).Gap(nodeRowGap)
			} else {
				ui.Box(c).Grow(1)
			}
		})
}

// pinListState wires the pinned list: its rows keep their identity by node
// name and reorder only while dragging is allowed.
func (v *View) pinListState(shown []api.AllNodeInfo, draggable bool) {
	v.PinListState.Key = func(i int) any { return shown[i].Name }
	if !draggable {
		v.PinListState.Reorder = nil
		return
	}
	v.PinListState.Reorder = func(rows []int, to int) {
		v.reorderPinned(shown, rows, to)
	}
}

// nodeListState wires the normal list the same way.
func (v *View) nodeListState(shown []api.AllNodeInfo, draggable bool) {
	v.NormalListState.Key = func(i int) any { return shown[i].Name }
	if !draggable {
		v.NormalListState.Reorder = nil
		return
	}
	v.NormalListState.Reorder = func(rows []int, to int) {
		v.reorderNormal(shown, rows, to)
	}
}

func (v *View) filterButton(c *ui.Context, label string, filter int) {
	t := c.Theme()
	selected := v.Filter == filter
	textColor := t.TextMuted
	weight := 400
	if selected {
		weight = 600
		textColor = t.Text
	}
	button := ui.ButtonBase(c).Label(label).Padding(4, 8).Radius(6).Children(func() {
		ui.Text(c, label).FontSize(11).FontWeight(weight).TextColor(textColor)
	})
	if selected {
		button = button.Background(t.Background).Border(1, t.Border.Alpha(0.4))
	}
	button.OnClick(func() {
		v.Filter = filter
		if filter == 1 {
			v.Nav = "favorites"
		} else {
			v.Nav = "nodes"
		}
		v.invalidate()
	})
}

func (v *View) nodeRow(c *ui.Context, n api.AllNodeInfo) {
	t := c.Theme()
	selected := n.Name == v.SelectedNode

	background := t.Background
	border := t.Border.Alpha(0.4)
	if selected {
		background = t.Accent.Alpha(0.08)
		border = t.Accent.Alpha(0.45)
	}

	ui.ButtonBase(c).FillWidth().
		Height(nodeRowHeight).
		Padding(10).
		Radius(8).
		Gap(10).
		AlignItems(ui.Center).
		Background(background).
		Border(1, border).
		Cursor(ui.CursorPointer).
		Children(func() {
			ui.Box(c).Size(34, 34).Radius(7).Background(t.Accent.Alpha(0.12)).Center().
				Children(func() {
					if img := v.nodeAvatar(v.ProxyURL, n.AvatarNormal); img != nil {
						ui.Image(c, img).Size(24, 24).Fit(ui.Contain)
					} else {
						ui.Icon(c, icons.Nodes).FontSize(17).TextColor(t.Accent)
					}
				})

			ui.Column(c).Grow(1).Gap(2).
				Children(func() {
					ui.Row(c).Gap(6).AlignItems(ui.Center).
						Children(func() {
							ui.Text(c, n.Title).FontSize(13).FontWeight(600).TextColor(t.Text).SingleLine()
							ui.Text(c, n.Name).FontSize(11).TextColor(t.TextMuted).SingleLine()
						})
					ui.Text(c, fmt.Sprintf("%s 主题 · %d 收藏", formatNumber(n.Topics), n.Stars)).
						FontSize(11).
						TextColor(t.TextMuted).
						SingleLine()
				})

			ui.Row(c).Gap(2).AlignItems(ui.Center).Children(func() {
				v.pinButton(c, n)
				if n.IsFavorite {
					v.favoriteStar(c, n, icons.StarFilled, ui.Hex("#F59E0B"))
				} else {
					v.favoriteStar(c, n, icons.Star, t.TextMuted)
				}
			})
		}).
		OnClick(func() {
			if v.SelectedNode != n.Name {
				// A new node starts its topic list at the top, not at the
				// previous node's scroll offset.
				v.TopicScroll.X, v.TopicScroll.Y = 0, 0
			}
			v.SelectedNode = n.Name
			v.Nav = "nodes"
			v.Filter = 0
			v.invalidate()
		})
}

// pinButton is the node row's pushpin: a button of its own, so clicking it
// pins or unpins the node without selecting the row around it.
func (v *View) pinButton(c *ui.Context, n api.AllNodeInfo) {
	t := c.Theme()
	pinned := v.isPinned(n.Name)
	icon := icons.Pin
	color := t.TextMuted
	if pinned {
		icon = icons.PinFilled
		color = t.Accent
	}

	button := ui.ButtonBase(c).Label("置顶节点").
		Size(24, 24).
		Radius(6).
		Center().
		Cursor(ui.CursorPointer)
	if button.Hovered() {
		button.Background(t.Accent.Alpha(0.08))
	}
	button.Children(func() {
		ui.Icon(c, icon).FontSize(14).TextColor(color)
	}).
		OnClick(func() {
			v.togglePin(n.Name)
		})
}

// favoriteStar is the node row's star: a button of its own, so clicking it
// toggles the favorite without selecting the row around it.
func (v *View) favoriteStar(c *ui.Context, n api.AllNodeInfo, icon *ui.SVG, color ui.Color) {
	t := c.Theme()
	star := ui.ButtonBase(c).Label("收藏节点").
		Size(28, 28).
		Radius(6).
		Center().
		Cursor(ui.CursorPointer)
	if star.Hovered() {
		star.Background(t.Accent.Alpha(0.08))
	}
	star.Children(func() {
		ui.Icon(c, icon).FontSize(15).TextColor(color)
	}).
		OnClick(func() {
			if _, err := v.FavoritesRepo.ToggleFavorite(n.Name, n.Title); err != nil {
				v.ShowToast("更新收藏失败")
				return
			}
			v.invalidate()
		})
}
