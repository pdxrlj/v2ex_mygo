package components

import (
	"fmt"
	"log"

	"mygo_demo/assets/img"
	"mygo_demo/internal/api"
	"mygo_demo/internal/database/repo"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// subscriptionWindow is the notification list window the app keeps open.
var subscriptionWindow *mygo.Window

// invalidateSubscriptionWindow redraws the list when a poll adds a batch.
func invalidateSubscriptionWindow() {
	if subscriptionWindow != nil {
		subscriptionWindow.Invalidate()
	}
}

// OpenSubscriptionWindow lists the new-reply notifications of the followed
// topics, marking them read.
func OpenSubscriptionWindow(v *View) {
	if subscriptionWindow != nil {
		subscriptionWindow.Close()
		subscriptionWindow = nil
	}

	closeSubscriptionPopup()
	_ = v.SubscriptionsRepo.MarkNotificationsRead()
	v.invalidate()
	invalidateTopicWindow()

	var win *mygo.Window
	win = mygo.NewWindow(mygo.WindowOptions{
		Title:           "订阅通知",
		Width:           560,
		Height:          620,
		MinWidth:        420,
		MinHeight:       360,
		BackgroundColor: "light-dark(#f5f6f8, #16181c)",
		Content: ui.View(func(c *ui.Context) {
			t := c.Theme()
			items, err := v.SubscriptionsRepo.Notifications(200)

			ui.Column(c).Fill().Padding(18).Gap(12).Background(t.Surface).Children(func() {
				ui.Row(c).FillWidth().AlignItems(ui.Center).Children(func() {
					ui.Text(c, "订阅通知").FontSize(18).FontWeight(700).TextColor(t.Text)
					ui.Spacer(c)
					ui.Text(c, "订阅主题有新回复时汇总到这里").
						FontSize(11).TextColor(t.TextMuted)
				})
				ui.Box(c).FillWidth().Height(1).Background(t.Border.Alpha(0.25))

				switch {
				case err != nil:
					ui.Text(c, "读取订阅通知失败。").FontSize(12).TextColor(t.TextMuted)
				case len(items) == 0:
					ui.Column(c).FillWidth().Grow(1).Center().Gap(8).Children(func() {
						ui.Text(c, "暂无订阅通知").FontSize(13).TextColor(t.TextMuted)
						ui.Text(c, "订阅的主题有新回复时会出现在这里。").
							FontSize(11).TextColor(t.TextMuted)
					})
				default:
					ui.Scroll(c).FillWidth().Grow(1).Children(func() {
						ui.Column(c).FillWidth().Children(func() {
							for i, n := range items {
								v.subscriptionNotificationRow(c, n, win)
								if i < len(items)-1 {
									ui.Box(c).FillWidth().Height(1).
										Background(t.Border.Alpha(0.15))
								}
							}
						})
					})
				}
			})
		}),
	})

	subscriptionWindow = win
	win.OnClosed(func() {
		if subscriptionWindow == win {
			subscriptionWindow = nil
		}
	})
	if err := win.SetIcon(img.LogoPNG); err != nil {
		log.Printf("subscription window icon: %v", err)
	}
}

// subscriptionNotificationRow draws one batch: which topic got how many
// replies, who replied last, and when. Clicking it opens the topic.
func (v *View) subscriptionNotificationRow(c *ui.Context, n repo.SubscriptionNotification, win *mygo.Window) {
	t := c.Theme()

	row := ui.ButtonBase(c).FillWidth().
		Padding(10, 8).
		Radius(8).
		Gap(10).
		AlignItems(ui.Start).
		Cursor(ui.CursorPointer)
	if row.Hovered() {
		row.Background(t.Surface)
	}

	row.Children(func() {
		ui.Column(c).Grow(1).Gap(3).Children(func() {
			title := n.TopicTitle
			if title == "" {
				title = fmt.Sprintf("主题 #%d", n.TopicID)
			}
			ui.Text(c, title).FontSize(13).FontWeight(600).
				TextColor(t.Text).MaxLines(1)

			meta := fmt.Sprintf("新增 %d 条回复 · 共 %d 条", n.NewReplies, n.TotalReplies)
			if n.LastReplyBy != "" {
				meta += " · 最后回复 " + n.LastReplyBy
			}
			ui.Text(c, meta).FontSize(11).TextColor(t.TextMuted)
		})
		ui.Text(c, timeAgo(n.CreatedAt.Unix())).FontSize(10).
			TextColor(t.TextMuted).SingleLine()
	})

	row.OnClick(func() {
		if win != nil {
			win.Close()
		}
		OpenTopicWindow(v, api.TopicInfo{ID: n.TopicID, Title: n.TopicTitle})
	})
}
