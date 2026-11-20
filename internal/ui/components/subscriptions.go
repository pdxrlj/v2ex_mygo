package components

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	"mygo_demo/internal/api"
	"mygo_demo/internal/database/repo"
	"mygo_demo/internal/ui/components/icons"

	"github.com/egoist/mygo/ui"
)

// subscriptionChange is one subscribed topic the last check found new
// replies in, and how many.
type subscriptionChange struct {
	topicID int
	title   string
	added   int
}

// The subscription check defaults to half an hour; five minutes is the
// shortest interval it offers, so five topics cost at most 60 requests an
// hour.
const (
	defaultSubscriptionPollInterval = 1800
	minSubscriptionPollInterval     = 300
	maxSubscriptionPollInterval     = 3600
)

// subscriptionPollOptions are the intervals the settings offer, in seconds.
var subscriptionPollOptions = []int{300, 600, 900, 1800, 3600}

// loadSubscriptionPollInterval reads the saved interval, with the default
// and the supported range applied.
func loadSubscriptionPollInterval(settings *repo.Settings) int {
	seconds := defaultSubscriptionPollInterval
	if value, ok, err := settings.Get(repo.SubscriptionPollIntervalKey); err == nil && ok {
		if n, err := strconv.Atoi(value); err == nil {
			seconds = min(max(n, minSubscriptionPollInterval), maxSubscriptionPollInterval)
		}
	}
	return seconds
}

// subscriptionPollInterval returns the applied interval.
func (v *View) subscriptionPollInterval() int {
	v.subMu.Lock()
	defer v.subMu.Unlock()
	return v.subPollSeconds
}

// wakeSubscriptionPoll lets the loop pick up a new subscription or interval
// at once.
func (v *View) wakeSubscriptionPoll() {
	if v.subWake == nil {
		return
	}
	select {
	case v.subWake <- struct{}{}:
	default:
	}
}

// pollSubscriptions checks the subscribed topics now and then for as long as
// the app runs. With no subscriptions it waits for one instead of waking on
// an interval, so an empty list costs nothing.
func (v *View) pollSubscriptions() {
	for {
		count, err := v.SubscriptionsRepo.Count()
		if err == nil && count == 0 {
			<-v.subWake
			continue
		}

		v.checkSubscriptions()

		timer := time.NewTimer(time.Duration(v.subscriptionPollInterval()) * time.Second)
		select {
		case <-timer.C:
		case <-v.subWake:
			timer.Stop()
		}
	}
}

// checkSubscriptions reads every subscribed topic once, in order, one
// request each. The topic's reply count alone tells whether replies
// arrived; the store adds one notification batch per change and ignores the
// same count again, so a repeated poll cannot duplicate a notification.
func (v *View) checkSubscriptions() {
	subs, err := v.SubscriptionsRepo.List()
	if err != nil || len(subs) == 0 {
		return
	}

	topics := api.NewTopic(v._client)
	var changes []subscriptionChange
	for _, sub := range subs {
		topic, err := topics.Get(sub.TopicID)
		if err != nil {
			_ = v.SubscriptionsRepo.MarkChecked(sub.TopicID)
			continue
		}
		newReplies, err := v.SubscriptionsRepo.ApplyCheck(
			sub.TopicID, topic.Title, topic.Replies, topic.LastReplyBy)
		if err == nil && newReplies > 0 {
			changes = append(changes, subscriptionChange{
				topicID: sub.TopicID,
				title:   subscriptionTopicName(sub.TopicID, topic.Title),
				added:   newReplies,
			})
		}
	}

	if len(changes) > 0 {
		v.invalidate()
		invalidateTopicWindow()
		invalidateSubscriptionWindow()
		showSubscriptionPopup(v, changes)
	}
}

// subscribeTopic adds a topic, with the reply count it shows now as the
// baseline, and lets the loop run at once.
func (v *View) subscribeTopic(topicID int, title string, replies int) error {
	if err := v.SubscriptionsRepo.Add(topicID, title, replies); err != nil {
		return err
	}
	v.wakeSubscriptionPoll()
	v.invalidate()
	invalidateTopicWindow()
	invalidateSubscriptionWindow()
	return nil
}

// unsubscribeTopic drops a topic and the notifications it made.
func (v *View) unsubscribeTopic(topicID int) error {
	if err := v.SubscriptionsRepo.Remove(topicID); err != nil {
		return err
	}
	v.invalidate()
	invalidateTopicWindow()
	invalidateSubscriptionWindow()
	return nil
}

// applySubscriptionPollInterval clamps and saves the check interval, and
// restarts the loop's wait with it.
func (v *View) applySubscriptionPollInterval(seconds int) {
	seconds = min(max(seconds, minSubscriptionPollInterval), maxSubscriptionPollInterval)

	if err := v.SettingsRepo.Set(repo.SubscriptionPollIntervalKey,
		strconv.Itoa(seconds)); err != nil {
		v.ShowToast("保存订阅查询间隔失败")
		return
	}

	v.subMu.Lock()
	v.subPollSeconds = seconds
	v.subMu.Unlock()
	v.wakeSubscriptionPoll()
	v.ShowToast("订阅查询间隔已更新")
	v.invalidate()
}

// subscriptionIntervalLabel names an interval in minutes or hours.
func subscriptionIntervalLabel(seconds int) string {
	switch {
	case seconds >= 3600 && seconds%3600 == 0:
		return strconv.Itoa(seconds/3600) + " 小时"
	case seconds%60 == 0:
		return strconv.Itoa(seconds/60) + " 分钟"
	}
	return strconv.Itoa(seconds) + " 秒"
}

// subscriptionSettingRow edits the subscription check interval and shows how
// many topics are followed.
func (v *View) subscriptionSettingRow(c *ui.Context) {
	t := c.Theme()
	interval := v.subscriptionPollInterval()
	count, _ := v.SubscriptionsRepo.Count()

	ui.Column(c).FillWidth().
		Padding(12, 14).
		Radius(8).
		Background(t.Surface).
		Gap(10).
		Children(func() {
			ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(8).Children(func() {
				ui.Text(c, "主题订阅").FontSize(13).
					FontWeight(600).TextColor(t.Text)
				ui.Text(c, fmt.Sprintf("已订阅 %d/%d", count, repo.MaxSubscriptions)).
					FontSize(10).TextColor(t.TextMuted)
				ui.Spacer(c)
				ui.Text(c, "新回复汇总到订阅通知").
					FontSize(11).TextColor(t.TextMuted)
			})

			ui.Text(c, "应用需保持运行；每个间隔按顺序查询一次订阅的主题。").
				FontSize(11).TextColor(t.TextMuted)

			ui.Row(c).FillWidth().AlignItems(ui.Center).Children(func() {
				ui.Text(c, "查询间隔").FontSize(12).TextColor(t.TextMuted).Grow(1)
				ui.Text(c, "每 "+subscriptionIntervalLabel(interval)+"检查一次").
					FontSize(11).TextColor(t.TextMuted)
			})

			ui.Row(c).FillWidth().Wrap().Gap(6).Children(func() {
				for _, option := range subscriptionPollOptions {
					v.subscriptionIntervalChip(c, option, interval)
				}
			})

			ui.Text(c, "默认 30 分钟，最小 5 分钟；最多订阅 5 个主题。").
				FontSize(11).TextColor(t.TextMuted)
		})
}

// subscriptionIntervalChip is one interval choice.
func (v *View) subscriptionIntervalChip(c *ui.Context, seconds, current int) {
	t := c.Theme()
	selected := seconds == current

	background, border := t.Background, t.Border.Alpha(0.4)
	textColor, weight := t.TextMuted, 400
	if selected {
		background, border = t.Accent.Alpha(0.12), t.Accent.Alpha(0.45)
		textColor, weight = t.Accent, 600
	}

	label := subscriptionIntervalLabel(seconds)
	ui.ButtonBase(c).Label("订阅查询间隔 "+label).
		Padding(5, 10).
		Radius(6).
		Background(background).
		Border(1, border).
		Children(func() {
			ui.Text(c, label).FontSize(11).FontWeight(weight).TextColor(textColor)
		}).
		OnClick(func() {
			v.applySubscriptionPollInterval(seconds)
		})
}

// subscriptionHeaderButton is the topic header's subscribe toggle, beside
// the open-in-browser button.
func (t *TopicWindow) subscriptionHeaderButton(c *ui.Context) {
	if t.owner == nil || t.topic == nil {
		return
	}
	th := c.Theme()

	_, subscribed, err := t.owner.SubscriptionsRepo.Get(t.topic.ID)
	if err != nil {
		return
	}

	label := "订阅"
	icon := icons.Bookmark
	background := th.Background
	border := th.Border.Alpha(0.4)
	foreground := th.Text
	if subscribed {
		label = "已订阅"
		icon = icons.BookmarkFilled
		background = th.Accent.Alpha(0.12)
		border = th.Accent.Alpha(0.45)
		foreground = th.Accent
	}

	button := ui.ButtonBase(c).Label("订阅主题").
		Padding(6, 10).
		Radius(6).
		Gap(5).
		AlignItems(ui.Center).
		Background(background).
		Border(1, border).
		Cursor(ui.CursorPointer)
	if button.Hovered() && !subscribed {
		button.Background(th.Surface)
	}
	button.Children(func() {
		ui.Icon(c, icon).FontSize(13).TextColor(foreground)
		ui.Text(c, label).FontSize(11).TextColor(foreground)
	}).
		OnClick(func() {
			if subscribed {
				if err := t.owner.unsubscribeTopic(t.topic.ID); err != nil {
					c.Toast("取消订阅失败")
					return
				}
				c.Toast("已取消订阅")
				return
			}

			err := t.owner.subscribeTopic(t.topic.ID, t.topic.Title, t.topic.Replies)
			switch {
			case errors.Is(err, repo.ErrSubscriptionLimit):
				c.Toast("最多只能订阅 5 个主题，请先取消一个")
			case err != nil:
				c.Toast("订阅失败")
			default:
				c.Toast("已订阅，新回复会出现在订阅通知")
			}
		})
}

// subscriptionsPanel is the sidebar's 订阅主题 page: every followed topic,
// its cancel action, and the entry to the notification list.
func (v *View) subscriptionsPanel(c *ui.Context) {
	t := c.Theme()
	subs, _ := v.SubscriptionsRepo.List()
	unread, _ := v.SubscriptionsRepo.UnreadNotificationCount()
	interval := v.subscriptionPollInterval()

	ui.Column(c).FillWidth().
		Padding(24).
		Radius(12).
		Background(t.Background).
		Border(1, t.Border.Alpha(0.4)).
		Gap(14).
		Children(func() {
			ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(8).Children(func() {
				ui.Text(c, "订阅主题").FontSize(20).FontWeight(700).TextColor(t.Text)
				ui.Box(c).Padding(1, 8).Radius(999).
					Background(t.Accent.Alpha(0.12)).
					Children(func() {
						ui.Text(c, fmt.Sprintf("%d/%d", len(subs), repo.MaxSubscriptions)).
							FontSize(10).TextColor(t.Accent)
					})
				ui.Spacer(c)
				ui.Text(c, "每 "+subscriptionIntervalLabel(interval)+"检查一次").
					FontSize(11).TextColor(t.TextMuted)

				label := "查看订阅通知"
				if unread > 0 {
					label = fmt.Sprintf("查看订阅通知（%d）", unread)
				}
				ui.ButtonBase(c).Label(label).
					Padding(5, 10).
					Radius(6).
					Background(t.Background).
					Border(1, t.Border.Alpha(0.4)).
					Children(func() {
						ui.Text(c, label).FontSize(11).TextColor(t.Text)
					}).
					OnClick(func() {
						OpenSubscriptionWindow(v)
					})
			})

			ui.Text(c, "在主题详情右上角点「订阅」关注新回复；应用需保持运行，最多 5 个。").
				FontSize(11).TextColor(t.TextMuted)

			ui.Box(c).FillWidth().Height(1).Background(t.Border.Alpha(0.25))

			if len(subs) == 0 {
				ui.Column(c).FillWidth().Padding(40).Center().Gap(8).Children(func() {
					ui.Icon(c, icons.Bookmark).FontSize(28).TextColor(t.TextMuted)
					ui.Text(c, "还没有订阅主题").FontSize(13).TextColor(t.TextMuted)
					ui.Text(c, "打开任意主题，点右上角的「订阅」按钮。").
						FontSize(11).TextColor(t.TextMuted)
				})
				return
			}

			ui.Column(c).FillWidth().Gap(8).Children(func() {
				for _, sub := range subs {
					v.subscriptionPanelRow(c, sub, interval)
				}
			})
		})
}

// subscriptionPanelRow shows one followed topic on the subscribe page.
func (v *View) subscriptionPanelRow(c *ui.Context, sub repo.Subscription, interval int) {
	t := c.Theme()
	title := sub.TopicTitle
	if title == "" {
		title = fmt.Sprintf("主题 #%d", sub.TopicID)
	}

	ui.Row(c).FillWidth().
		Padding(10, 8).
		Radius(8).
		Background(t.Surface).
		Gap(10).
		AlignItems(ui.Center).
		Children(func() {
			ui.Column(c).Grow(1).Gap(2).Children(func() {
				ui.Text(c, title).FontSize(13).FontWeight(600).
					TextColor(t.Text).MaxLines(1)

				meta := fmt.Sprintf("上次 %d 条回复 · 每 %s 检查",
					sub.LastReplies, subscriptionIntervalLabel(interval))
				if !sub.CheckedAt.IsZero() {
					meta += " · 上次检查 " + timeAgo(sub.CheckedAt.Unix())
				}
				ui.Text(c, meta).FontSize(11).TextColor(t.TextMuted)
			})

			v.subscriptionPanelButton(c, "打开", func() {
				OpenTopicWindow(v, api.TopicInfo{
					ID:    sub.TopicID,
					Title: sub.TopicTitle,
				})
			})
			v.subscriptionPanelButton(c, "取消订阅", func() {
				if err := v.unsubscribeTopic(sub.TopicID); err != nil {
					c.Toast("取消订阅失败")
					return
				}
				c.Toast("已取消订阅")
			})
		})
}

// subscriptionPanelButton is one small action of the subscribe page.
func (v *View) subscriptionPanelButton(c *ui.Context, label string, action func()) {
	t := c.Theme()
	ui.ButtonBase(c).Label(label).
		Padding(4, 8).
		Radius(6).
		Background(t.Background).
		Border(1, t.Border.Alpha(0.4)).
		Children(func() {
			ui.Text(c, label).FontSize(11).TextColor(t.TextMuted)
		}).
		OnClick(action)
}
