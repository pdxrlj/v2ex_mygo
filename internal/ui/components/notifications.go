package components

import (
	"fmt"
	"regexp"
	"strconv"
	"time"

	"mygo_demo/internal/api"
	"mygo_demo/internal/database/repo"
	"mygo_demo/internal/ui/components/icons"

	"github.com/egoist/mygo/ui"
)

// The scheduled reminder refresh defaults to a minute, stays at least a
// minute so it stays well inside the API's 600-requests-per-hour limit, and
// tops out at an hour.
const (
	defaultNotificationsPollInterval = 60
	minNotificationsPollInterval     = 60
	maxNotificationsPollInterval     = 3600
)

// notificationTopicHref matches the topic link of a reminder's text; group 1
// is the topic id.
var notificationTopicHref = regexp.MustCompile(`href="/t/([0-9]+)`)

// notificationTopicTitle matches the topic title inside that link; group 1
// is the title.
var notificationTopicTitle = regexp.MustCompile(`class="topic-link">([^<]*)</a>`)

// loadNotificationsSeen reads the newest reminder id the member has read,
// and whether one was saved at all.
func loadNotificationsSeen(settings *repo.Settings) (int, bool) {
	value, ok, err := settings.Get(repo.NotificationsSeenKey)
	if err != nil || !ok {
		return 0, false
	}
	seen, _ := strconv.Atoi(value)
	return seen, true
}

// loadNotificationPollSettings reads the saved schedule, with the interval
// clamped to the supported range and a default when none was saved.
func loadNotificationPollSettings(settings *repo.Settings) (bool, int) {
	enabled := true
	if value, ok, err := settings.Get(repo.NotificationsPollKey); err == nil && ok {
		enabled = value != "0"
	}

	seconds := defaultNotificationsPollInterval
	if value, ok, err := settings.Get(repo.NotificationsPollIntervalKey); err == nil && ok {
		if n, err := strconv.Atoi(value); err == nil && n >= minNotificationsPollInterval {
			seconds = min(n, maxNotificationsPollInterval)
		}
	}
	return enabled, seconds
}

// setNav switches the visible section, marking the messages page read once
// the user leaves it.
func (v *View) setNav(id string) {
	previous := v.Nav
	v.Nav = id
	if previous == "messages" && id != "messages" {
		v.markNotificationsSeen()
	}
}

// pollNotifications refreshes the reminders on the configured schedule,
// waiting for a settings change while the schedule is off. The API has no
// push channel, so this is the only way to see new reminders.
func (v *View) pollNotifications() {
	for {
		enabled, seconds := v.notificationPollSettings()
		if !enabled {
			<-v.notifWake
			continue
		}

		v.fetchNotifications()

		timer := time.NewTimer(time.Duration(seconds) * time.Second)
		select {
		case <-timer.C:
		case <-v.notifWake:
			timer.Stop()
		}
	}
}

// notificationPollSettings returns the applied schedule.
func (v *View) notificationPollSettings() (bool, int) {
	v.notifMu.Lock()
	defer v.notifMu.Unlock()
	return v.notifPollEnabled, v.notifPollSeconds
}

// wakeNotificationPoll lets the poll pick up a schedule change at once.
func (v *View) wakeNotificationPoll() {
	if v.notifWake == nil {
		return
	}
	select {
	case v.notifWake <- struct{}{}:
	default:
	}
}

// applyNotificationPollEnabled saves the switch and lets the poll start or
// stop following it.
func (v *View) applyNotificationPollEnabled() {
	enabled := v.NotifPollEnabled

	if err := v.SettingsRepo.Set(repo.NotificationsPollKey,
		notificationPollSetting(enabled)); err != nil {
		// The switch already shows the new state; put it back when the save
		// fails, so the panel never claims a setting that is not stored.
		v.NotifPollEnabled = !enabled
		v.ShowToast("保存定时刷新设置失败")
		v.invalidate()
		return
	}

	v.notifMu.Lock()
	v.notifPollEnabled = enabled
	v.notifMu.Unlock()

	v.wakeNotificationPoll()
	v.ShowToast("定时刷新设置已更新")
	v.invalidate()
}

// applyNotificationPollInterval clamps the chosen interval to the supported
// range and saves it for the running poll.
func (v *View) applyNotificationPollInterval(seconds int) {
	switch {
	case seconds < minNotificationsPollInterval:
		seconds = minNotificationsPollInterval
	case seconds > maxNotificationsPollInterval:
		seconds = maxNotificationsPollInterval
	}

	if err := v.SettingsRepo.Set(repo.NotificationsPollIntervalKey,
		strconv.Itoa(seconds)); err != nil {
		v.ShowToast("保存刷新间隔失败")
		return
	}

	v.notifMu.Lock()
	v.notifPollSeconds = seconds
	v.notifMu.Unlock()

	v.wakeNotificationPoll()
	v.ShowToast("刷新间隔已改为每 " + notificationIntervalLabel(seconds) + "一次")
	v.invalidate()
}

// notificationPollSetting is how the settings store a switch.
func notificationPollSetting(enabled bool) string {
	if enabled {
		return "1"
	}
	return "0"
}

// notificationIntervals are the refresh intervals the settings panel
// offers, in seconds.
var notificationIntervals = []int{60, 120, 300, 600, 1800, 3600}

// notificationIntervalLabel names an interval in the unit that reads best.
func notificationIntervalLabel(seconds int) string {
	switch {
	case seconds >= 3600 && seconds%3600 == 0:
		return strconv.Itoa(seconds/3600) + " 小时"
	case seconds >= 60 && seconds%60 == 0:
		return strconv.Itoa(seconds/60) + " 分钟"
	}
	return strconv.Itoa(seconds) + " 秒"
}

// isPresetNotificationInterval tells whether seconds is one of the offered
// intervals, so a saved custom value gets a chip of its own.
func isPresetNotificationInterval(seconds int) bool {
	for _, option := range notificationIntervals {
		if option == seconds {
			return true
		}
	}
	return false
}

// notificationPollSettingRow edits the scheduled refresh: a switch turning
// it on or off, then the interval choices it waits between reads.
func (v *View) notificationPollSettingRow(c *ui.Context) {
	t := c.Theme()
	enabled, seconds := v.notificationPollSettings()

	ui.Column(c).FillWidth().
		Padding(12, 14).
		Radius(8).
		Background(t.Surface).
		Gap(10).
		Children(func() {
			ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(12).Children(func() {
				ui.Column(c).Grow(1).Gap(3).Children(func() {
					ui.Text(c, "定时刷新通知").FontSize(13).
						FontWeight(600).TextColor(t.Text)
					ui.Text(c, "关闭后不再自动拉取消息，仍可手动刷新").
						FontSize(11).TextColor(t.TextMuted)
				})

				toggle := ui.Switch(c, &v.NotifPollEnabled).
					Label("定时刷新通知")
				if toggle.Changed() {
					v.applyNotificationPollEnabled()
				}
			})

			ui.Box(c).FillWidth().Height(1).Background(t.Border.Alpha(0.2))

			ui.Row(c).FillWidth().AlignItems(ui.Center).Children(func() {
				ui.Text(c, "刷新间隔").FontSize(12).
					TextColor(t.TextMuted).Grow(1)
				status := "每 " + notificationIntervalLabel(seconds) + "检查一次"
				if !enabled {
					status = "自动刷新已关闭"
				}
				ui.Text(c, status).FontSize(11).TextColor(t.TextMuted)
			})

			ui.Row(c).FillWidth().Wrap().Gap(6).Children(func() {
				for _, option := range notificationIntervals {
					v.notificationIntervalChip(c, option, seconds)
				}
				// A saved custom interval keeps a chip of its own, so the
				// current setting always shows as chosen.
				if !isPresetNotificationInterval(seconds) {
					v.notificationIntervalChip(c, seconds, seconds)
				}
			})

			ui.Text(c, "间隔越短提醒越及时；最小 60 秒，API 限流为每小时 600 次请求。").
				FontSize(11).TextColor(t.TextMuted)
		})
}

// notificationIntervalChip is one interval choice: the current one takes the
// accent color, the others stay quiet until chosen.
func (v *View) notificationIntervalChip(c *ui.Context, seconds, current int) {
	t := c.Theme()
	selected := seconds == current

	background := t.Background
	border := t.Border.Alpha(0.4)
	textColor := t.TextMuted
	weight := 400
	if selected {
		background = t.Accent.Alpha(0.12)
		border = t.Accent.Alpha(0.45)
		textColor = t.Accent
		weight = 600
	}

	ui.ButtonBase(c).Label("刷新间隔 "+notificationIntervalLabel(seconds)).
		Padding(5, 10).
		Radius(6).
		Background(background).
		Border(1, border).
		Children(func() {
			ui.Text(c, notificationIntervalLabel(seconds)).FontSize(11).
				FontWeight(weight).TextColor(textColor)
		}).
		OnClick(func() {
			v.applyNotificationPollInterval(seconds)
		})
}

// fetchNotifications reads the newest page of reminders in the background
// and redraws the window with what changed. A failure keeps the reminders
// already shown and only records the error.
func (v *View) fetchNotifications() {
	v.notifMu.Lock()
	if v.notifLoading {
		v.notifMu.Unlock()
		return
	}
	v.notifLoading = true
	v.notifMu.Unlock()

	items, err := api.NewNotifications(v._client).List(1)

	v.notifMu.Lock()
	v.notifLoading = false
	v.notifLoaded = true
	baseline := false
	if err != nil {
		v.notifErr = "获取消息失败"
	} else {
		v.notifErr = ""
		v.notifications = items
		if !v.notifSeenSet {
			// First successful read: the reminders already on the site are
			// read, so the badge counts what arrives from now on. An empty
			// page still sets the baseline, or a new account's first
			// reminder would arrive without a badge.
			newest := 0
			for _, n := range items {
				if n.ID > newest {
					newest = n.ID
				}
			}
			v.notifSeenID = newest
			v.notifSeenSet = true
			baseline = true
		}
	}
	v.notifMu.Unlock()

	if baseline {
		_ = v.SettingsRepo.Set(repo.NotificationsSeenKey,
			strconv.Itoa(v.notificationSeenID()))
	}

	v.invalidate()
}

// notificationsSnapshot copies what the messages page draws, so the poll can
// replace the slice without racing the frame.
func (v *View) notificationsSnapshot() ([]api.Notification, bool, bool, string) {
	v.notifMu.Lock()
	defer v.notifMu.Unlock()

	items := append([]api.Notification(nil), v.notifications...)
	return items, v.notifLoading, v.notifLoaded, v.notifErr
}

// unreadNotifications counts the reminders newer than the newest one the
// member has read. The poll holds the first page only, so the count stops at
// its size.
func (v *View) unreadNotifications() int {
	v.notifMu.Lock()
	defer v.notifMu.Unlock()

	unread := 0
	for _, n := range v.notifications {
		if n.ID > v.notifSeenID {
			unread++
		}
	}
	return unread
}

// unreadNotificationLabel is the sidebar badge's text: empty when nothing is
// new, "9+" past the first page's reach.
func unreadNotificationLabel(n int) string {
	switch {
	case n <= 0:
		return ""
	case n > 9:
		return "9+"
	}
	return strconv.Itoa(n)
}

// notificationSeenID returns the newest reminder id the member has read.
func (v *View) notificationSeenID() int {
	v.notifMu.Lock()
	defer v.notifMu.Unlock()
	return v.notifSeenID
}

// markNotificationsSeen records the newest reminder as read and saves it, so
// the badge clears and the next start keeps the newest page read.
func (v *View) markNotificationsSeen() {
	v.notifMu.Lock()
	seen := v.notifSeenID
	for _, n := range v.notifications {
		if n.ID > seen {
			seen = n.ID
		}
	}
	changed := seen != v.notifSeenID
	if changed {
		v.notifSeenID = seen
		v.notifSeenSet = true
	}
	v.notifMu.Unlock()

	if changed {
		_ = v.SettingsRepo.Set(repo.NotificationsSeenKey, strconv.Itoa(seen))
	}
}

// notificationsPanel draws the messages page: the unread state and a refresh
// button on top, then the newest reminders as rows.
func (v *View) notificationsPanel(c *ui.Context) {
	t := c.Theme()
	items, loading, loaded, loadErr := v.notificationsSnapshot()
	unread := v.unreadNotifications()
	enabled, seconds := v.notificationPollSettings()

	hint := "自动刷新已关闭"
	if enabled {
		hint = fmt.Sprintf("每 %d 秒自动刷新", seconds)
	}

	ui.Column(c).FillWidth().
		Padding(24).
		Radius(12).
		Background(t.Background).
		Border(1, t.Border.Alpha(0.4)).
		Gap(14).
		Children(func() {
			ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(8).Children(func() {
				ui.Text(c, "消息通知").FontSize(20).FontWeight(700).TextColor(t.Text)
				if unread > 0 {
					ui.Box(c).Padding(1, 8).Radius(999).
						Background(t.Accent.Alpha(0.12)).
						Children(func() {
							ui.Text(c, "未读 "+strconv.Itoa(unread)).FontSize(10).
								TextColor(t.Accent)
						})
				}
				ui.Spacer(c)
				ui.Text(c, hint).FontSize(11).TextColor(t.TextMuted)
				v.notificationsRefreshButton(c, loading)
			})

			ui.Box(c).FillWidth().Height(1).Background(t.Border.Alpha(0.25))

			switch {
			case len(items) == 0 && (loading || (!loaded && enabled)):
				ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(8).Children(func() {
					ui.Spinner(c).Label("正在获取消息")
					ui.Text(c, "正在获取消息…").FontSize(12).TextColor(t.TextMuted)
				})
			case len(items) == 0 && loadErr != "":
				ui.Text(c, loadErr+"。").FontSize(12).TextColor(t.TextMuted)
			case len(items) == 0 && !loaded:
				ui.Text(c, "自动刷新已关闭，点右上角的刷新按钮获取消息。").
					FontSize(12).TextColor(t.TextMuted)
			case len(items) == 0:
				ui.Column(c).FillWidth().Padding(48, 24).Center().Gap(8).Children(func() {
					ui.Icon(c, icons.Bell).FontSize(28).TextColor(t.TextMuted)
					ui.Text(c, "暂无消息").FontSize(13).TextColor(t.TextMuted)
					ui.Text(c, "有人回复你、@你或感谢你时，提醒会出现在这里。").
						FontSize(11).TextColor(t.TextMuted)
				})
			default:
				ui.Column(c).FillWidth().Children(func() {
					for i, n := range items {
						v.notificationRow(c, n)
						if i < len(items)-1 {
							ui.Box(c).FillWidth().Height(1).
								Background(t.Border.Alpha(0.15))
						}
					}
				})
			}
		})
}

// notificationsRefreshButton reloads the reminders now, and shows a spinner
// while the request is in flight.
func (v *View) notificationsRefreshButton(c *ui.Context, loading bool) {
	t := c.Theme()

	if loading {
		ui.Spinner(c).Label("正在刷新消息")
		return
	}
	ui.ButtonBase(c).Label("刷新消息").
		Size(28, 28).
		Radius(6).
		Center().
		Cursor(ui.CursorPointer).
		Children(func() {
			ui.Icon(c, icons.Refresh).FontSize(14).TextColor(t.TextMuted)
		}).
		OnClick(func() {
			go v.fetchNotifications()
		})
}

// notificationRow draws one reminder: the actor, the reminder's sentence,
// the related reply when there is one, and the time. Clicking it opens the
// topic it points at.
func (v *View) notificationRow(c *ui.Context, n api.Notification) {
	t := c.Theme()
	actor := n.Member.Username
	summary := plainText(n.Text)
	detail := plainText(n.PayloadRendered)
	topicID, topicTitle := notificationTopic(n.Text)
	unread := n.ID > v.notificationSeenID()

	row := ui.ButtonBase(c).FillWidth().
		Padding(10, 8).
		Radius(8).
		Gap(12).
		AlignItems(ui.Start).
		Cursor(ui.CursorPointer)
	if row.Hovered() {
		row.Background(t.Surface)
	}

	row.Children(func() {
		ui.Avatar(c, actor, nil).Size(30, 30)

		ui.Column(c).Grow(1).Gap(4).Children(func() {
			ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(6).Children(func() {
				if unread {
					ui.Box(c).Size(6, 6).Radius(3).Background(t.Accent)
				}
				ui.Text(c, summary).FontSize(13).TextColor(t.Text).
					MaxLines(2).Grow(1)
				ui.Text(c, timeAgo(n.Created)).FontSize(10).
					TextColor(t.TextMuted).SingleLine()
			})
			if detail != "" {
				ui.Text(c, detail).FontSize(12).TextColor(t.TextMuted).MaxLines(2)
			}
		})
	})

	row.OnClick(func() {
		if topicID <= 0 {
			v.ShowToast("无法定位这条提醒对应的主题")
			return
		}
		title := topicTitle
		if title == "" {
			title = "主题详情"
		}
		OpenTopicWindow(v, api.TopicInfo{ID: topicID, Title: title})
	})
}

// notificationTopic returns the topic a reminder points at: the id of its
// topic link and the link's title. A reminder without a topic link returns
// zero and an empty title.
func notificationTopic(text string) (id int, title string) {
	if m := notificationTopicHref.FindStringSubmatch(text); len(m) == 2 {
		id, _ = strconv.Atoi(m[1])
	}
	if m := notificationTopicTitle.FindStringSubmatch(text); len(m) == 2 {
		title = plainText(m[1])
	}
	return id, title
}
