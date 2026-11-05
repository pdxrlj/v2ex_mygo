package components

import (
	"mygo_demo/internal/ui/components/icons"

	"github.com/egoist/mygo/ui"
)

// NavSidebar draws the vertical navigation: the browse group, a gap, then
// the account group, as the reference's icon rail does.
func (v *View) NavSidebar(c *ui.Context) {
	t := c.Theme()
	width, _ := c.Size()
	compact := width < 1080

	// The counts come from the same sources as the filters: the whole node
	// listing for "节点列表" and the local favorites table for "收藏节点".
	// A failed listing read leaves the node count blank rather than zero.
	nodeCount := ""
	if n := v.nodeTotal(); n > 0 {
		nodeCount = formatNumber(n)
	}
	favoriteCount := formatNumber(v.favoritesCount())

	sidebar := ui.Column(c).FillHeight().
		Background(t.Surface).
		BorderWidth(0, 1, 0, 0).
		BorderLinearGradient(fadeDivider(t))
	if compact {
		sidebar.Width(64).Padding(6).Gap(4)
	} else {
		sidebar.Width(220).Padding(10, 8)
	}

	sidebar.Children(func() {
		if compact {
			v.navItem(c, "nodes", "节点列表", icons.Nodes, nodeCount, true)
			v.navItem(c, "favorites", "收藏节点", icons.Star, favoriteCount, true)
			ui.Box(c).FillWidth().Height(1).Background(t.Border.Alpha(0.25)).MarginY(6)
			v.navItem(c, "profile", "我的资料", icons.User, "", true)
			v.navItem(c, "settings", "系统设置", icons.Gear, "", true)
			return
		}

		ui.Text(c, "浏览").FontSize(11).FontWeight(600).TextColor(t.TextMuted).Margin(6, 6, 4, 4)

		v.navItem(c, "nodes", "节点列表", icons.Nodes, nodeCount, false)
		v.navItem(c, "favorites", "收藏节点", icons.Star, favoriteCount, false)

		ui.Box(c).Height(14)

		ui.Text(c, "账号").FontSize(11).FontWeight(600).
			TextColor(t.TextMuted).
			Margin(6, 6, 4, 4)

		v.navItem(c, "profile", "我的资料", icons.User, "", false)
		v.navItem(c, "settings", "系统设置", icons.Gear, "", false)
	})
}

func (v *View) navItem(c *ui.Context, id, label string, icon *ui.SVG, count string, compact bool) {
	t := c.Theme()
	selected := v.Nav == id
	background := ui.Transparent
	foreground := t.Text
	iconColor := t.TextMuted
	if selected {
		background = t.Accent.Alpha(0.10)
		iconColor = t.Accent
	}

	item := ui.ButtonBase(c).
		Cursor(ui.CursorPointer).
		Label(label).FillWidth().Radius(6).
		AlignItems(ui.Center).
		Background(background)
	if compact {
		item.Height(38).Center().Children(func() {
			ui.Icon(c, icon).FontSize(18).TextColor(iconColor)
		})
	} else {
		item.Height(34).Padding(0, 8).Gap(8).Children(func() {
			ui.Icon(c, icon).FontSize(16).TextColor(iconColor)
			ui.Text(c, label).FontSize(13).TextColor(foreground).Grow(1).SingleLine()
			if count != "" {
				ui.Text(c, count).FontSize(11).TextColor(t.TextMuted)
			}
		})
	}

	item.OnClick(func() {
		v.Nav = id
		switch id {
		case "favorites":
			v.Filter = 1
		case "nodes":
			v.Filter = 0
		}
		v.invalidate()
	})
}
