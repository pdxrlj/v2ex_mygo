package api

import "strconv"

// notificationsPath lists the signed-in member's reminders, newest first.
const notificationsPath = "v2/notifications"

// Notification is one reminder of the signed-in member: someone replied,
// mentioned, thanked or starred. Text is HTML naming the actor and linking
// the topic; Payload is the related reply's text and PayloadRendered its
// HTML. The API has no read flag, so a client decides which reminders are
// new.
type Notification struct {
	ID              int        `json:"id"`
	MemberID        int        `json:"member_id"`
	ForMemberID     int        `json:"for_member_id"`
	Text            string     `json:"text"`
	Payload         *string    `json:"payload"`
	PayloadRendered string     `json:"payload_rendered"`
	Created         int64      `json:"created"`
	Member          MemberInfo `json:"member"`
}

// Notifications accesses the reminder endpoints.
type Notifications struct {
	Client *Client
}

// NewNotifications returns a Notifications service backed by client.
func NewNotifications(client *Client) *Notifications {
	return &Notifications{Client: client}
}

type notificationsResponse struct {
	v2Response
	Result []Notification `json:"result"`
}

// List returns one page of the member's reminders, newest first. Pages
// start at 1.
func (n *Notifications) List(page int) ([]Notification, error) {
	if page < 1 {
		page = 1
	}

	var body notificationsResponse
	var apiErr v2Response
	resp, err := n.Client.R().
		SetResult(&body).
		SetError(&apiErr).
		SetQueryParam("p", strconv.Itoa(page)).
		Get(notificationsPath)
	if err != nil {
		return nil, err
	}
	if err := body.check(resp, apiErr); err != nil {
		return nil, err
	}

	return body.Result, nil
}
