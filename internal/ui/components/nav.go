package components

import (
	"mygo_demo/assets/img"
	"mygo_demo/internal/ui/components/icons"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// NavComments draws the window's header: the custom window controls, the
// logo, the breadcrumb, the search field and the tools, left to right.
func (v *View) NavComments(c *ui.Context) {
	t := c.Theme()
	ui.Row(c).FillWidth().Height(52).Padding(0, 16).Gap(10).
		AlignItems(ui.Center).
		Background(t.Surface).
		DragWindow().
		Children(func() {
			v.windowControls(c)

			ui.Image(c, img.Logo).Size(24, 24).Fit(ui.Contain)
			ui.Text(c, "V2EX").FontSize(14).FontWeight(700).TextColor(t.Text)

			ui.Row(c).Gap(6).AlignItems(ui.Center).MarginStart(12).
				Children(func() {
					ui.Text(c, "全部节点").FontSize(12).TextColor(t.TextMuted)
					ui.Text(c, "›").FontSize(12).TextColor(t.TextMuted)
					ui.Text(c, v.navTitle()).FontSize(12).FontWeight(600).
						TextColor(t.Text)
				})

			ui.Spacer(c)

			v.headerButton(c, v.CurrentTheme(), "切换外观", func() {
				if mygo.Theme.IsDark() {
					mygo.Theme.SetSource(mygo.ThemeLight)
				} else {
					mygo.Theme.SetSource(mygo.ThemeDark)
				}
			})

			v.headerButton(c, icons.Gear, "设置", func() {
				v.Nav = "settings"
				v.invalidate()
			})

			ui.ButtonBase(c).Label("账户").Size(28, 28).Radius(999).
				Children(func() {
					v.userAvatar(c)
				}).
				OnClick(func() {
					v.Nav = "profile"
					v.invalidate()
				})
		})
}

// userAvatar shows the account's picture, or its initials until the
// download finishes.
func (v *View) userAvatar(c *ui.Context) {
	user := v.loadUser()
	if user == nil {
		ui.Avatar(c, "V2EX", nil).Size(28, 28)
		return
	}

	if img := v.nodeAvatarBitmap(v.ProxyURL, user.AvatarNormal); img != nil {
		ui.Avatar(c, user.Username, img).Size(28, 28)
		return
	}
	ui.Avatar(c, user.Username, nil).Size(28, 28)
}

// windowControls draws the minimize, maximize and close buttons the page
// owns, since the system's are hidden.
func (v *View) windowControls(c *ui.Context) {
	ui.Row(c).Gap(4).AlignItems(ui.Center).
		Children(func() {
			v.windowButton(c, icons.Minimize, "最小化", func() {
				if v.Win != nil {
					v.Win.Minimize()
				}
			})
			v.windowButton(c, icons.Maximize, "最大化", func() {
				if v.Win != nil {
					v.Win.ToggleMaximize()
				}
			})
			v.windowButton(c, icons.Close, "关闭", func() {
				if v.Win != nil {
					v.Win.Close()
				}
			})
		})
}

func (v *View) windowButton(c *ui.Context, icon *ui.SVG, label string, action func()) {
	t := c.Theme()
	var button ui.Element
	button = ui.ButtonBase(c).Label(label).Size(38, 32).Radius(6).
		Cursor(ui.CursorPointer).
		Draw(func(p *ui.Painter, r ui.Rect) {
			if button.Hovered() {
				p.Fill(r, t.SurfaceHover, 6)
			}
		}).
		Children(func() {
			ui.Icon(c, icon).FontSize(16).TextColor(t.TextMuted)
		})
	button.OnClick(action)
}

func (v *View) headerButton(c *ui.Context, icon *ui.SVG, label string, action func()) {
	t := c.Theme()
	ui.ButtonBase(c).Label(label).Size(32, 32).Radius(6).
		Children(func() {
			ui.Icon(c, icon).FontSize(17).TextColor(t.TextMuted)
		}).
		OnClick(action)
}
