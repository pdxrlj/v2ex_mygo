package components

import "github.com/egoist/mygo/ui"

// ShowToast shows msg near the bottom of the window for a few seconds.
//
// The UI library queues toasts on the frame's context, so this must run
// while the view is building, as clicks and other children of Draw do;
// calls made at other times are dropped. v.ctx is captured at the start of
// Draw and is stable for the window's life.
func (v *View) ShowToast(msg string) {
	if v.ctx == nil {
		return
	}
	v.ctx.Toast(msg)
}

// ShowMessage opens the message drawer with msg, replacing any message
// showing. Call it while the view is building, as a click handler does;
// v.invalidate requests the frame that then draws it.
func (v *View) ShowMessage(msg string) {
	v.MessageText = msg
	v.invalidate()
}

// Message draws the message drawer while one is set: a panel sliding in
// from the right over a dimmed backdrop, which Escape or a press outside
// it closes. Draw calls it last, so it shows above the content.
func (v *View) Message(c *ui.Context) {
	if v.MessageText == "" {
		return
	}

	t := c.Theme()
	ui.Overlay(c, func() {
		back := ui.Row(c).Absolute().Left(0).Top(0).Right(0).Bottom(0).
			Justify(ui.End).
			Background(ui.RGBA(0, 0, 0, 0.3)).
			Modal()

		back.Children(func() {
			panel := ui.Column(c).Key("message").
				Width(320).FillHeight().
				Padding(16).
				Background(t.Surface).
				Role(ui.RoleDialog).Label("消息").
				Transition(ui.ElementTransition{
					Enter: &ui.Motion{X: 320},
					Exit:  &ui.Motion{X: 320},
				})

			panel.Children(func() {
				ui.Text(c, v.MessageText).FontSize(14)
			})

			if panel.PressedOutside() || back.OverlayShortcut(0, ui.KeyEscape) {
				v.MessageText = ""
				v.invalidate()
			}
		})
	})
}
