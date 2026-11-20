package components

import (
	"mygo_demo/assets/img"

	"github.com/egoist/mygo"
)

// appTray is the notification area icon the app keeps for its life, and
// trayQuitting lets the main window close for real when the tray quits.
var (
	appTray      *mygo.Tray
	trayQuitting bool
)

// SetupTray adds the notification area icon: a click shows the main window,
// a right click opens its menu, and closing the main window hides it here
// instead of quitting. It returns an error when the platform refuses the
// icon; the caller then leaves the window closing as it was.
func SetupTray(v *View, win *mygo.Window) error {
	tray, err := mygo.NewTray(mygo.TrayOptions{
		ID:      "main",
		Icon:    img.TrayPNG,
		ToolTip: "V2EX",
	})
	if err != nil {
		return err
	}
	appTray = tray

	show := func() {
		win.Show()
		win.Focus()
	}

	tray.OnClick(show)
	tray.OnRightClick(func() {
		menu := mygo.NewMenu([]*mygo.MenuItem{
			{Label: "显示主窗口", Click: func(*mygo.MenuItem, *mygo.Window) {
				show()
			}},
			mygo.Separator(),
			{Label: "订阅通知", Click: func(*mygo.MenuItem, *mygo.Window) {
				show()
				OpenSubscriptionWindow(v)
			}},
			{Label: "订阅主题", Click: func(*mygo.MenuItem, *mygo.Window) {
				show()
				v.setNav("subscriptions")
				v.invalidate()
			}},
			{Label: "系统设置", Click: func(*mygo.MenuItem, *mygo.Window) {
				show()
				v.setNav("settings")
				v.invalidate()
			}},
			mygo.Separator(),
			{Label: "退出 V2EX", Click: func(*mygo.MenuItem, *mygo.Window) {
				quitFromTray()
			}},
		})
		tray.SetMenu(menu)
		tray.PopUpMenu()
		tray.SetMenu(nil)
	})

	// The close button and Alt+F4 keep the app in the tray; the tray's quit
	// lets the window close for real.
	win.OnClose(func(e *mygo.CloseEvent) {
		if trayQuitting {
			return
		}
		e.PreventDefault()
		win.Hide()
	})

	return nil
}

// quitFromTray quits the app for real, tray icon included.
func quitFromTray() {
	trayQuitting = true
	if appTray != nil {
		appTray.Destroy()
		appTray = nil
	}
	mygo.App.Quit()
}
