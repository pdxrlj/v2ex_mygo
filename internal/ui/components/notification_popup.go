package components

import (
	"fmt"
	"sync"
	"time"

	"mygo_demo/assets/img"
	"mygo_demo/internal/api"
	"mygo_demo/internal/ui/components/icons"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

const (
	subscriptionPopupWidth  = 380
	subscriptionPopupHeight = 152
	// subscriptionPopupLife is how long the card stays before it puts itself
	// away, as a system notification would.
	subscriptionPopupLife = 12 * time.Second
)

// subscriptionPopup is the notification card currently shown, if any.
var subscriptionPopup struct {
	sync.Mutex
	win *mygo.Window
}

// showSubscriptionPopup shows the themed card in the corner of the screen:
// its body opens the topic, its cross and 去除通知 put it away, and it goes
// on its own after a while.
func showSubscriptionPopup(v *View, changes []subscriptionChange) {
	if len(changes) == 0 {
		return
	}
	closeSubscriptionPopup()

	var win *mygo.Window
	win = mygo.NewWindow(mygo.WindowOptions{
		Title:           "V2EX 订阅提醒",
		Width:           subscriptionPopupWidth,
		Height:          subscriptionPopupHeight,
		Frameless:       true,
		CornerRadius:    12,
		AlwaysOnTop:     true,
		SkipTaskbar:     true,
		DisableResize:   true,
		DisableMove:     true,
		DisableMinimize: true,
		DisableMaximize: true,
		BackgroundColor: "light-dark(#f5f6f8, #16181c)",
		Content: ui.View(func(c *ui.Context) {
			t := c.Theme()

			ui.Column(c).Fill().Padding(14).Gap(8).
				Background(t.Background).
				Border(1, t.Border.Alpha(0.4)).
				Radius(12).
				Children(func() {
					ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
						ui.Image(c, img.Logo).Size(22, 22).Fit(ui.Contain)
						ui.Text(c, "V2EX 订阅提醒").FontSize(13).
							FontWeight(700).TextColor(t.Text).Grow(1)

						remove := ui.ButtonBase(c).Label("去除通知").
							Size(22, 22).Radius(6).Center().
							Cursor(ui.CursorPointer)
						remove.Children(func() {
							ui.Icon(c, icons.Close).FontSize(12).TextColor(t.TextMuted)
						})
						remove.OnClick(func() {
							closeSubscriptionPopupWindow(win)
						})
					})

					body := ui.ButtonBase(c).Label("查看详情").
						FillWidth().Padding(0, 2).AlignItems(ui.Start).
						Cursor(ui.CursorPointer)
					body.Children(func() {
						ui.Text(c, subscriptionPopupBody(changes)).FontSize(12).
							TextColor(t.TextMuted).MaxLines(2)
					})
					body.OnClick(func() {
						closeSubscriptionPopupWindow(win)
						openSubscriptionTarget(v, changes)
					})

					ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(8).Children(func() {
						ui.Spacer(c)

						removeText := ui.ButtonBase(c).Label("去除通知").
							Padding(3, 8).Radius(6).Background(t.Surface).
							Cursor(ui.CursorPointer)
						removeText.Children(func() {
							ui.Text(c, "去除通知").FontSize(11).TextColor(t.TextMuted)
						})
						removeText.OnClick(func() {
							closeSubscriptionPopupWindow(win)
						})

						view := ui.ButtonBase(c).Label("查看详情").
							Padding(4, 10).Radius(6).Background(t.Accent).
							Cursor(ui.CursorPointer)
						view.Children(func() {
							ui.Text(c, "查看详情").FontSize(11).TextColor(ui.Hex("#FFFFFF"))
						})
						view.OnClick(func() {
							closeSubscriptionPopupWindow(win)
							openSubscriptionTarget(v, changes)
						})
					})
				})
		}),
	})

	subscriptionPopup.Lock()
	subscriptionPopup.win = win
	subscriptionPopup.Unlock()

	win.OnClosed(func() {
		subscriptionPopup.Lock()
		if subscriptionPopup.win == win {
			subscriptionPopup.win = nil
		}
		subscriptionPopup.Unlock()
	})

	d := mygo.Screen.PrimaryDisplay()
	win.SetPosition(
		d.WorkArea.X+d.WorkArea.Width-subscriptionPopupWidth-16,
		d.WorkArea.Y+d.WorkArea.Height-subscriptionPopupHeight-16,
	)
	win.ShowInactive()

	time.AfterFunc(subscriptionPopupLife, func() {
		closeSubscriptionPopupWindow(win)
	})
}

// closeSubscriptionPopup puts the current card away, if one shows.
func closeSubscriptionPopup() {
	subscriptionPopup.Lock()
	win := subscriptionPopup.win
	subscriptionPopup.win = nil
	subscriptionPopup.Unlock()
	if win != nil {
		win.Close()
	}
}

// closeSubscriptionPopupWindow puts away win only if it is still the card
// shown, so a timer of a replaced card does not close its successor.
func closeSubscriptionPopupWindow(win *mygo.Window) {
	subscriptionPopup.Lock()
	if subscriptionPopup.win != win {
		subscriptionPopup.Unlock()
		return
	}
	subscriptionPopup.win = nil
	subscriptionPopup.Unlock()
	win.Close()
}

// subscriptionPopupBody names the new replies the card reports.
func subscriptionPopupBody(changes []subscriptionChange) string {
	if len(changes) == 1 {
		title := subscriptionTopicName(changes[0].topicID, changes[0].title)
		return fmt.Sprintf("《%s》新增 %d 条回复", title, changes[0].added)
	}
	return fmt.Sprintf("%d 个订阅主题有新回复，打开列表查看。", len(changes))
}

// subscriptionTopicName is the topic's title, or its id when the API named
// none.
func subscriptionTopicName(topicID int, title string) string {
	if title != "" {
		return title
	}
	return fmt.Sprintf("主题 #%d", topicID)
}

// openSubscriptionTarget opens the topic for one change, and the list for
// several.
func openSubscriptionTarget(v *View, changes []subscriptionChange) {
	if len(changes) == 1 {
		OpenTopicWindow(v, api.TopicInfo{
			ID:    changes[0].topicID,
			Title: changes[0].title,
		})
		return
	}
	OpenSubscriptionWindow(v)
}
