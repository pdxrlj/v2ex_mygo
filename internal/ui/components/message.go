package components

// ShowToast shows msg near the bottom of the window for a few seconds, as
// the outcome of what the user just did.
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
