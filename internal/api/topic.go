package api

import (
	"fmt"
	"net/url"
	"regexp"
	"strconv"
)

const topicPath = "v2/topics"

// latestTopicsPath is a legacy V2EX API 1.0 endpoint that returns the latest
// topics in one unpaginated response. It is absent from the API 2.0
// documentation and may disappear; cache the result.
const latestTopicsPath = "topics/latest.json"

// hotTopicsPath is the API 1.0 endpoint for the current hot topics. It has
// the same plain-array, unpaginated shape as latestTopicsPath.
const hotTopicsPath = "topics/hot.json"

// nodePagePath is the public node page listing the same topics as the API
// 2.0 node endpoint, with each row's poster name and avatar.
const nodePagePath = "/go/"

// nodeRowPattern matches the opening tag of one topic row on a node page;
// group 1 is the topic id.
var nodeRowPattern = regexp.MustCompile(`<div class="cell from_[0-9]+ t_([0-9]+)"`)

// nodePosterPattern matches the poster link and avatar image at the start of
// a topic row; group 1 is the username, group 2 the avatar URL.
var nodePosterPattern = regexp.MustCompile(`<a href="/member/([^"]+)"><img src="([^"]+)"`)

// Topic exposes the V2EX topic endpoints: the API 2.0 detail and replies,
// and the legacy API 1.0 latest and hot topic feeds.
type Topic struct {
	Client *Client
}

func NewTopic(client *Client) *Topic {
	return &Topic{Client: client}
}

// MemberInfo is the member summary embedded in topic and reply responses.
// API 2.0 returns a single avatar; the legacy API 1.0 returns the three
// sizes instead, so both shapes are kept here.
type MemberInfo struct {
	ID           int    `json:"id"`
	Username     string `json:"username"`
	Bio          string `json:"bio"`
	Website      string `json:"website"`
	GitHub       string `json:"github"`
	URL          string `json:"url"`
	Avatar       string `json:"avatar"`
	AvatarMini   string `json:"avatar_mini"`
	AvatarNormal string `json:"avatar_normal"`
	AvatarLarge  string `json:"avatar_large"`
	Created      int64  `json:"created"`
	Pro          int64  `json:"pro"`
}

// AvatarURL returns the member's picture in the most suitable size, or an
// empty string when the response carried none. Rows are small, so the
// normal avatar is preferred over the large API 2.0 one.
func (m *MemberInfo) AvatarURL() string {
	switch {
	case m == nil:
		return ""
	case m.AvatarNormal != "":
		return m.AvatarNormal
	case m.Avatar != "":
		return m.Avatar
	case m.AvatarLarge != "":
		return m.AvatarLarge
	}
	return m.AvatarMini
}

// Supplement is an addendum to a topic.
type Supplement struct {
	ID              int    `json:"id"`
	Content         string `json:"content"`
	ContentRendered string `json:"content_rendered"`
	Syntax          int    `json:"syntax"`
	Created         int64  `json:"created"`
}

// TopicInfo is the topic object returned by the topic endpoints. Member,
// Node and Supplements are only present on the detail endpoint; list
// responses leave them nil/empty.
type TopicInfo struct {
	ID              int          `json:"id"`
	Title           string       `json:"title"`
	Content         string       `json:"content"`
	ContentRendered string       `json:"content_rendered"`
	Syntax          int          `json:"syntax"`
	URL             string       `json:"url"`
	Replies         int          `json:"replies"`
	Stars           int          `json:"stars"`
	Thanks          int          `json:"thanks"`
	LastReplyBy     string       `json:"last_reply_by"`
	Created         int64        `json:"created"`
	LastModified    int64        `json:"last_modified"`
	LastTouched     int64        `json:"last_touched"`
	Member          *MemberInfo  `json:"member,omitempty"`
	Node            *NodeInfo    `json:"node,omitempty"`
	Supplements     []Supplement `json:"supplements,omitempty"`
}

// ReplyInfo is one reply in a topic.
type ReplyInfo struct {
	ID              int         `json:"id"`
	Content         string      `json:"content"`
	ContentRendered string      `json:"content_rendered"`
	Created         int64       `json:"created"`
	Member          *MemberInfo `json:"member,omitempty"`
}

type topicResponse struct {
	v2Response
	Result TopicInfo `json:"result"`
}

type topicRepliesResponse struct {
	v2Response
	Result     []ReplyInfo `json:"result"`
	Pagination Pagination  `json:"pagination"`
}

// Get returns the details of a topic.
func (t *Topic) Get(id int) (*TopicInfo, error) {
	if id < 1 {
		return nil, fmt.Errorf("invalid topic id: %d", id)
	}

	var body topicResponse
	var apiErr v2Response
	resp, err := t.Client.R().
		SetResult(&body).
		SetError(&apiErr).
		Get(topicPath + "/" + strconv.Itoa(id))
	if err != nil {
		return nil, err
	}
	if err := body.check(resp, apiErr); err != nil {
		return nil, err
	}

	return &body.Result, nil
}

// Replies returns a page of replies for a topic. Pages start at 1.
func (t *Topic) Replies(id, page int) ([]ReplyInfo, Pagination, error) {
	if id < 1 {
		return nil, Pagination{}, fmt.Errorf("invalid topic id: %d", id)
	}
	if page < 1 {
		page = 1
	}

	var body topicRepliesResponse
	var apiErr v2Response
	resp, err := t.Client.R().
		SetResult(&body).
		SetError(&apiErr).
		SetQueryParam("p", strconv.Itoa(page)).
		Get(topicPath + "/" + strconv.Itoa(id) + "/replies")
	if err != nil {
		return nil, Pagination{}, err
	}
	if err := body.check(resp, apiErr); err != nil {
		return nil, Pagination{}, err
	}

	return body.Result, body.Pagination, nil
}

// Latest returns the latest topics from the legacy V2EX API 1.0 feed. The
// response is a plain array, unlike the API 2.0 envelope, and the endpoint
// is not paginated. Cache the result rather than requesting it each frame.
func (t *Topic) Latest() ([]TopicInfo, error) {
	var topics []TopicInfo
	resp, err := t.Client.R().
		SetResult(&topics).
		Get(latestTopicsPath)
	if err != nil {
		return nil, err
	}
	if resp.IsError() {
		return nil, fmt.Errorf("v2ex api error: %s", resp.Status())
	}

	return topics, nil
}

// Hot returns the current hot topics from the legacy V2EX API 1.0 feed. Like
// Latest, the response is a plain array and the endpoint is not paginated.
func (t *Topic) Hot() ([]TopicInfo, error) {
	var topics []TopicInfo
	resp, err := t.Client.R().
		SetResult(&topics).
		Get(hotTopicsPath)
	if err != nil {
		return nil, err
	}
	if resp.IsError() {
		return nil, fmt.Errorf("v2ex api error: %s", resp.Status())
	}

	return topics, nil
}

// NodePagePosters returns the poster of every topic listed on a node's
// public page, keyed by topic id. The API 2.0 node listing carries no
// member, and the legacy listing only covers the ten newest topics, while
// the page carries all of its rows' posters. Pages start at 1.
func (t *Topic) NodePagePosters(nodeName string, page int) (map[int]*MemberInfo, error) {
	if nodeName == "" {
		return nil, fmt.Errorf("node name is required")
	}
	if page < 1 {
		page = 1
	}

	site := t.Client.SiteURL
	if site == "" {
		site = siteURL
	}

	resp, err := t.Client.R().
		SetQueryParam("p", strconv.Itoa(page)).
		Get(site + nodePagePath + url.PathEscape(nodeName))
	if err != nil {
		return nil, err
	}
	if resp.IsError() {
		return nil, fmt.Errorf("v2ex page error: %s", resp.Status())
	}

	return parseNodePagePosters(resp.String()), nil
}

// parseNodePagePosters cuts the page into topic rows and reads the poster
// from each; rows without a member picture are skipped and keep the list's
// fallback avatar.
func parseNodePagePosters(page string) map[int]*MemberInfo {
	rows := nodeRowPattern.FindAllStringSubmatchIndex(page, -1)
	posters := make(map[int]*MemberInfo, len(rows))
	for i, row := range rows {
		id, err := strconv.Atoi(page[row[2]:row[3]])
		if err != nil {
			continue
		}

		end := len(page)
		if i+1 < len(rows) {
			end = rows[i+1][0]
		}
		poster := nodePosterPattern.FindStringSubmatch(page[row[1]:end])
		if poster == nil {
			continue
		}
		posters[id] = &MemberInfo{Username: poster[1], Avatar: poster[2]}
	}

	return posters
}
