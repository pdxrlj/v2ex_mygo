package repo

import (
	"database/sql"
	"errors"
	"fmt"

	"mygo_demo/internal/database"
)

// ProxyKey is the setting that holds the proxy URL. A stored empty value
// means a direct connection; a missing row means it was never set.
const ProxyKey = "proxy"

// TokenKey is the setting that holds the V2EX Personal Access Token saved
// from the settings panel. A missing row means the token comes from the
// environment or .env file.
const TokenKey = "token"

// NodeLayoutKey is the setting that holds the node list's saved arrangement:
// its pinned nodes and the custom order of the rest, as JSON.
const NodeLayoutKey = "node_layout"

// NotificationsSeenKey is the setting that holds the newest reminder id the
// member has read; reminders newer than it count as unread.
const NotificationsSeenKey = "notifications_seen"

// NotificationsPollKey turns the scheduled reminder refresh on ("1") or off
// ("0").
const NotificationsPollKey = "notifications_poll"

// NotificationsPollIntervalKey is the setting that holds how often the
// scheduled refresh reads the reminders, in seconds.
const NotificationsPollIntervalKey = "notifications_poll_interval"

// WebCookieKey is the setting that holds the V2EX web session cookie, DPAPI
// encrypted and base64 encoded.
const WebCookieKey = "web_cookie"

// AutoSignInKey turns the daily bonus check-in on ("1") or off ("0").
const AutoSignInKey = "auto_sign_in"

// LastSignInKey is the day the daily bonus was last claimed, as YYYY-MM-DD,
// so the automatic check-in runs once a day.
const LastSignInKey = "last_sign_in"

// SubscriptionPollIntervalKey is the setting that holds how often the
// subscribed topics are checked for new replies, in seconds.
const SubscriptionPollIntervalKey = "subscription_poll_interval"

// TopicSplitKey is the setting that holds the topic window's divider
// position: the percentage of the width the topic pane takes.
const TopicSplitKey = "topic_split"

// Settings is the repository of the app's small key-value settings.
type Settings struct {
	db *sql.DB
}

// NewSettings returns a settings repository backed by conn.
func NewSettings(conn *database.DB) *Settings {
	return &Settings{db: conn.SQL()}
}

// Get returns the value stored under key and whether a row exists, so a
// caller can tell an explicit empty value from an unset one.
func (s *Settings) Get(key string) (string, bool, error) {
	var value string
	err := s.db.QueryRow(`SELECT value FROM settings WHERE key = ?`, key).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("read setting %q: %w", key, err)
	}
	return value, true, nil
}

// Set stores value under key, replacing what was there.
func (s *Settings) Set(key, value string) error {
	_, err := s.db.Exec(`INSERT INTO settings (key, value)
		VALUES (?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value`,
		key, value)
	if err != nil {
		return fmt.Errorf("write setting %q: %w", key, err)
	}
	return nil
}
