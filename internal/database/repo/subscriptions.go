package repo

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"mygo_demo/internal/database"
)

// MaxSubscriptions is the most topics one member may subscribe to.
const MaxSubscriptions = 5

// maxSubscriptionNotifications is how many notification rows are kept; older
// ones are pruned as new ones arrive, so the table cannot grow without end.
const maxSubscriptionNotifications = 200

// ErrSubscriptionLimit reports that the subscription list is full.
var ErrSubscriptionLimit = errors.New("subscription limit reached")

// Subscription is one topic the member follows for new replies.
type Subscription struct {
	TopicID     int
	TopicTitle  string
	LastReplies int
	CreatedAt   time.Time
	CheckedAt   time.Time
}

// SubscriptionNotification is one batch of new replies found for a
// subscribed topic.
type SubscriptionNotification struct {
	ID           int64
	TopicID      int
	TopicTitle   string
	NewReplies   int
	TotalReplies int
	LastReplyBy  string
	CreatedAt    time.Time
	Read         bool
}

// Subscriptions is the repository of the member's topic subscriptions and
// the notifications their new replies made.
type Subscriptions struct {
	db *sql.DB
}

// NewSubscriptions returns a subscriptions repository backed by conn.
func NewSubscriptions(conn *database.DB) *Subscriptions {
	return &Subscriptions{db: conn.SQL()}
}

// Add subscribes to a topic, with replies as the count the topic has now so
// only later replies count as new. Subscribing to a topic already followed
// only updates its title. Adding a sixth topic returns ErrSubscriptionLimit.
func (r *Subscriptions) Add(topicID int, title string, replies int) error {
	if topicID < 1 {
		return fmt.Errorf("invalid topic id: %d", topicID)
	}

	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("begin subscribe: %w", err)
	}
	defer tx.Rollback()

	var exists int
	err = tx.QueryRow(`SELECT 1 FROM subscriptions WHERE topic_id = ?`, topicID).
		Scan(&exists)
	switch {
	case err == nil:
		if _, err := tx.Exec(
			`UPDATE subscriptions SET topic_title = ? WHERE topic_id = ?`,
			title, topicID); err != nil {
			return fmt.Errorf("update subscription %d: %w", topicID, err)
		}
		return tx.Commit()
	case !errors.Is(err, sql.ErrNoRows):
		return fmt.Errorf("read subscription %d: %w", topicID, err)
	}

	var count int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM subscriptions`).Scan(&count); err != nil {
		return fmt.Errorf("count subscriptions: %w", err)
	}
	if count >= MaxSubscriptions {
		return ErrSubscriptionLimit
	}

	if _, err := tx.Exec(`INSERT INTO subscriptions
		(topic_id, topic_title, last_replies, created_at, checked_at)
		VALUES (?, ?, ?, ?, 0)`,
		topicID, title, replies, time.Now().Unix()); err != nil {
		return fmt.Errorf("subscribe to %d: %w", topicID, err)
	}
	return tx.Commit()
}

// Remove drops a subscription and the notifications it made, so a topic the
// member no longer follows leaves nothing behind.
func (r *Subscriptions) Remove(topicID int) error {
	if topicID < 1 {
		return fmt.Errorf("invalid topic id: %d", topicID)
	}

	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("begin unsubscribe: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.Exec(
		`DELETE FROM subscription_notifications WHERE topic_id = ?`, topicID); err != nil {
		return fmt.Errorf("delete notifications of %d: %w", topicID, err)
	}
	if _, err := tx.Exec(`DELETE FROM subscriptions WHERE topic_id = ?`, topicID); err != nil {
		return fmt.Errorf("unsubscribe %d: %w", topicID, err)
	}
	return tx.Commit()
}

// Get returns one subscription and whether it exists.
func (r *Subscriptions) Get(topicID int) (Subscription, bool, error) {
	var (
		sub     Subscription
		created int64
		checked int64
	)
	err := r.db.QueryRow(`SELECT topic_id, topic_title, last_replies, created_at, checked_at
		FROM subscriptions WHERE topic_id = ?`, topicID).
		Scan(&sub.TopicID, &sub.TopicTitle, &sub.LastReplies, &created, &checked)
	if errors.Is(err, sql.ErrNoRows) {
		return Subscription{}, false, nil
	}
	if err != nil {
		return Subscription{}, false, fmt.Errorf("read subscription %d: %w", topicID, err)
	}
	sub.CreatedAt = time.Unix(created, 0)
	sub.CheckedAt = time.Unix(checked, 0)
	return sub, true, nil
}

// List returns every subscription, most recently added first.
func (r *Subscriptions) List() ([]Subscription, error) {
	rows, err := r.db.Query(`SELECT topic_id, topic_title, last_replies, created_at, checked_at
		FROM subscriptions ORDER BY created_at DESC, topic_id`)
	if err != nil {
		return nil, fmt.Errorf("list subscriptions: %w", err)
	}
	defer rows.Close()

	var out []Subscription
	for rows.Next() {
		var (
			sub     Subscription
			created int64
			checked int64
		)
		if err := rows.Scan(&sub.TopicID, &sub.TopicTitle, &sub.LastReplies,
			&created, &checked); err != nil {
			return nil, fmt.Errorf("scan subscription: %w", err)
		}
		sub.CreatedAt = time.Unix(created, 0)
		sub.CheckedAt = time.Unix(checked, 0)
		out = append(out, sub)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list subscriptions: %w", err)
	}
	return out, nil
}

// Count returns how many topics are subscribed.
func (r *Subscriptions) Count() (int, error) {
	var count int
	if err := r.db.QueryRow(`SELECT COUNT(*) FROM subscriptions`).Scan(&count); err != nil {
		return 0, fmt.Errorf("count subscriptions: %w", err)
	}
	return count, nil
}

// ApplyCheck records a topic read: when replies grew since the last read, the
// baseline moves, a notification batch is added, and the number of new
// replies is returned. Reading the same count again adds nothing, so a poll
// that repeats cannot duplicate a notification.
func (r *Subscriptions) ApplyCheck(topicID int, title string, replies int, lastReplyBy string) (int, error) {
	tx, err := r.db.Begin()
	if err != nil {
		return 0, fmt.Errorf("begin subscription check: %w", err)
	}
	defer tx.Rollback()

	var last int
	err = tx.QueryRow(`SELECT last_replies FROM subscriptions WHERE topic_id = ?`,
		topicID).Scan(&last)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil // unsubscribed while the request was in flight
	}
	if err != nil {
		return 0, fmt.Errorf("read subscription %d: %w", topicID, err)
	}

	now := time.Now().Unix()
	switch {
	case replies == last:
		if _, err := tx.Exec(`UPDATE subscriptions
			SET topic_title = ?, checked_at = ? WHERE topic_id = ?`,
			title, now, topicID); err != nil {
			return 0, fmt.Errorf("touch subscription %d: %w", topicID, err)
		}
		return 0, tx.Commit()
	case replies < last:
		// Replies were removed: move the baseline down without notifying,
		// so replies after the removal are counted from the new count.
		if _, err := tx.Exec(`UPDATE subscriptions
			SET topic_title = ?, last_replies = ?, checked_at = ? WHERE topic_id = ?`,
			title, replies, now, topicID); err != nil {
			return 0, fmt.Errorf("lower subscription %d: %w", topicID, err)
		}
		return 0, tx.Commit()
	}

	added := replies - last
	if _, err := tx.Exec(`UPDATE subscriptions
		SET topic_title = ?, last_replies = ?, checked_at = ? WHERE topic_id = ?`,
		title, replies, now, topicID); err != nil {
		return 0, fmt.Errorf("update subscription %d: %w", topicID, err)
	}
	if _, err := tx.Exec(`INSERT INTO subscription_notifications
		(topic_id, topic_title, new_replies, total_replies, last_reply_by, created_at, read)
		VALUES (?, ?, ?, ?, ?, ?, 0)`,
		topicID, title, added, replies, lastReplyBy, now); err != nil {
		return 0, fmt.Errorf("add subscription notification: %w", err)
	}
	if _, err := tx.Exec(`DELETE FROM subscription_notifications
		WHERE id NOT IN (SELECT id FROM subscription_notifications
			ORDER BY id DESC LIMIT ?)`, maxSubscriptionNotifications); err != nil {
		return 0, fmt.Errorf("prune subscription notifications: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return added, nil
}

// MarkChecked records that a topic was read, for a read that could not get
// the topic.
func (r *Subscriptions) MarkChecked(topicID int) error {
	if _, err := r.db.Exec(`UPDATE subscriptions SET checked_at = ?
		WHERE topic_id = ?`, time.Now().Unix(), topicID); err != nil {
		return fmt.Errorf("touch subscription %d: %w", topicID, err)
	}
	return nil
}

// Notifications returns the newest notification batches, at most limit.
func (r *Subscriptions) Notifications(limit int) ([]SubscriptionNotification, error) {
	if limit <= 0 {
		limit = maxSubscriptionNotifications
	}
	rows, err := r.db.Query(`SELECT id, topic_id, topic_title, new_replies,
		total_replies, last_reply_by, created_at, read
		FROM subscription_notifications
		ORDER BY created_at DESC, id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("list subscription notifications: %w", err)
	}
	defer rows.Close()

	var out []SubscriptionNotification
	for rows.Next() {
		var (
			n       SubscriptionNotification
			created int64
			read    int
		)
		if err := rows.Scan(&n.ID, &n.TopicID, &n.TopicTitle, &n.NewReplies,
			&n.TotalReplies, &n.LastReplyBy, &created, &read); err != nil {
			return nil, fmt.Errorf("scan subscription notification: %w", err)
		}
		n.CreatedAt = time.Unix(created, 0)
		n.Read = read != 0
		out = append(out, n)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list subscription notifications: %w", err)
	}
	return out, nil
}

// UnreadNotificationCount returns how many batches the member has not read.
func (r *Subscriptions) UnreadNotificationCount() (int, error) {
	var count int
	if err := r.db.QueryRow(
		`SELECT COUNT(*) FROM subscription_notifications WHERE read = 0`).Scan(&count); err != nil {
		return 0, fmt.Errorf("count unread subscription notifications: %w", err)
	}
	return count, nil
}

// MarkNotificationsRead marks every notification batch as read.
func (r *Subscriptions) MarkNotificationsRead() error {
	if _, err := r.db.Exec(
		`UPDATE subscription_notifications SET read = 1 WHERE read = 0`); err != nil {
		return fmt.Errorf("mark subscription notifications read: %w", err)
	}
	return nil
}
