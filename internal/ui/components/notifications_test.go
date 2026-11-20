package components

import (
	"testing"

	"mygo_demo/internal/api"
)

// TestNotificationTopic checks that the topic link of a reminder's HTML is
// read back as its id and title.
func TestNotificationTopic(t *testing.T) {
	text := `<a href="/member/immango" target="_blank"><strong>immango</strong></a>` +
		` 在 <a href="/t/1133733#reply3" class="topic-link">各位大佬，` +
		`目前语音识别最好的模型是那个</a> 里回复了你`

	id, title := notificationTopic(text)
	if id != 1133733 {
		t.Errorf("topic id = %d, want 1133733", id)
	}
	if title != "各位大佬，目前语音识别最好的模型是那个" {
		t.Errorf("topic title = %q", title)
	}

	// A reminder without a topic link names no topic.
	if id, title := notificationTopic("系统提醒"); id != 0 || title != "" {
		t.Errorf("reminder without a link = (%d, %q), want (0, \"\")", id, title)
	}
}

// TestUnreadNotifications checks that only reminders newer than the read id
// count as unread.
func TestUnreadNotifications(t *testing.T) {
	v := &View{
		notifications: []api.Notification{{ID: 10}, {ID: 9}, {ID: 5}},
		notifSeenID:   9,
	}
	if got := v.unreadNotifications(); got != 1 {
		t.Errorf("unreadNotifications() = %d, want 1", got)
	}

	v.notifSeenID = 10
	if got := v.unreadNotifications(); got != 0 {
		t.Errorf("unreadNotifications() after reading all = %d, want 0", got)
	}
}

// TestUnreadNotificationLabel checks the badge text, capped at "9+".
func TestUnreadNotificationLabel(t *testing.T) {
	cases := map[int]string{0: "", 3: "3", 9: "9", 10: "9+"}
	for n, want := range cases {
		if got := unreadNotificationLabel(n); got != want {
			t.Errorf("unreadNotificationLabel(%d) = %q, want %q", n, got, want)
		}
	}
}

// TestNotificationIntervalLabel checks the units the interval chips show.
func TestNotificationIntervalLabel(t *testing.T) {
	cases := map[int]string{
		60:   "1 分钟",
		120:  "2 分钟",
		600:  "10 分钟",
		1800: "30 分钟",
		3600: "1 小时",
		90:   "90 秒",
	}
	for seconds, want := range cases {
		if got := notificationIntervalLabel(seconds); got != want {
			t.Errorf("notificationIntervalLabel(%d) = %q, want %q", seconds, got, want)
		}
	}
}
