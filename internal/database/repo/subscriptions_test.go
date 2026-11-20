package repo

import (
	"errors"
	"path/filepath"
	"testing"

	"mygo_demo/internal/database"
)

// newTestSubscriptions returns a repository backed by a database file in the
// test's temp dir.
func newTestSubscriptions(t *testing.T) *Subscriptions {
	t.Helper()

	conn, err := database.Open(filepath.Join(t.TempDir(), "mygo.db"))
	if err != nil {
		t.Fatalf("database.Open: %v", err)
	}
	t.Cleanup(func() { conn.Close() })

	return NewSubscriptions(conn)
}

// TestSubscriptionLimit checks the five-topic cap and that adding a topic
// already followed is not a sixth.
func TestSubscriptionLimit(t *testing.T) {
	subs := newTestSubscriptions(t)

	for i := 1; i <= MaxSubscriptions; i++ {
		if err := subs.Add(i, "topic", 0); err != nil {
			t.Fatalf("Add(%d): %v", i, err)
		}
	}
	if err := subs.Add(MaxSubscriptions+1, "topic", 0); !errors.Is(err, ErrSubscriptionLimit) {
		t.Fatalf("Add sixth = %v, want ErrSubscriptionLimit", err)
	}
	if err := subs.Add(1, "updated", 0); err != nil {
		t.Fatalf("re-Add existing: %v", err)
	}

	count, err := subs.Count()
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if count != MaxSubscriptions {
		t.Errorf("Count = %d, want %d", count, MaxSubscriptions)
	}
}

// TestApplyCheckDedupes checks that a growing topic makes one notification,
// and reading the same count again makes none.
func TestApplyCheckDedupes(t *testing.T) {
	subs := newTestSubscriptions(t)
	if err := subs.Add(7, "keyboard", 3); err != nil {
		t.Fatalf("Add: %v", err)
	}

	added, err := subs.ApplyCheck(7, "keyboard", 5, "alice")
	if err != nil {
		t.Fatalf("ApplyCheck with new replies: %v", err)
	}
	if added != 2 {
		t.Errorf("added = %d, want 2", added)
	}

	added, err = subs.ApplyCheck(7, "keyboard", 5, "alice")
	if err != nil {
		t.Fatalf("ApplyCheck repeating: %v", err)
	}
	if added != 0 {
		t.Errorf("added on repeat = %d, want 0", added)
	}

	items, err := subs.Notifications(10)
	if err != nil {
		t.Fatalf("Notifications: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("got %d notifications, want 1", len(items))
	}
	if items[0].NewReplies != 2 || items[0].TotalReplies != 5 ||
		items[0].LastReplyBy != "alice" {
		t.Errorf("notification = %+v", items[0])
	}

	unread, err := subs.UnreadNotificationCount()
	if err != nil {
		t.Fatalf("UnreadNotificationCount: %v", err)
	}
	if unread != 1 {
		t.Errorf("unread = %d, want 1", unread)
	}
	if err := subs.MarkNotificationsRead(); err != nil {
		t.Fatalf("MarkNotificationsRead: %v", err)
	}
	unread, err = subs.UnreadNotificationCount()
	if err != nil {
		t.Fatalf("UnreadNotificationCount after read: %v", err)
	}
	if unread != 0 {
		t.Errorf("unread after read = %d, want 0", unread)
	}
}

// TestApplyCheckHandlesRemovedReplies checks that deleted replies lower the
// baseline without a notification, and that later growth counts from there.
func TestApplyCheckHandlesRemovedReplies(t *testing.T) {
	subs := newTestSubscriptions(t)
	if err := subs.Add(11, "topic", 5); err != nil {
		t.Fatalf("Add: %v", err)
	}

	if added, err := subs.ApplyCheck(11, "topic", 3, "alice"); err != nil || added != 0 {
		t.Fatalf("ApplyCheck with removed replies = (%d, %v), want (0, nil)", added, err)
	}
	if added, err := subs.ApplyCheck(11, "topic", 4, "bob"); err != nil || added != 1 {
		t.Fatalf("ApplyCheck after removal = (%d, %v), want (1, nil)", added, err)
	}

	items, err := subs.Notifications(10)
	if err != nil {
		t.Fatalf("Notifications: %v", err)
	}
	if len(items) != 1 || items[0].NewReplies != 1 {
		t.Errorf("notifications = %+v, want one with 1 reply", items)
	}
}

// TestRemoveSubscriptionDropsNotifications checks that unsubscribing leaves
// neither the topic nor its notification behind.
func TestRemoveSubscriptionDropsNotifications(t *testing.T) {
	subs := newTestSubscriptions(t)
	if err := subs.Add(9, "topic", 0); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if _, err := subs.ApplyCheck(9, "topic", 4, "bob"); err != nil {
		t.Fatalf("ApplyCheck: %v", err)
	}

	if err := subs.Remove(9); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if _, ok, err := subs.Get(9); err != nil || ok {
		t.Errorf("Get after remove = (%v, %v), want absent", ok, err)
	}
	items, err := subs.Notifications(10)
	if err != nil {
		t.Fatalf("Notifications: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("got %d notifications after remove, want 0", len(items))
	}
}
