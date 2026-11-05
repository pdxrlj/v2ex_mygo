package api

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestTopicGet(t *testing.T) {
	var path string
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		writeJSON(t, w, http.StatusOK, topicResponse{
			v2Response: v2Response{Success: true, Message: "Topic details found"},
			Result: TopicInfo{
				ID:    1,
				Title: "Introducing Project Babel 2.0",
				Member: &MemberInfo{
					ID:       1,
					Username: "Livid",
				},
				Node: &NodeInfo{
					ID:    1,
					Name:  "babel",
					Title: "Project Babel",
				},
				Supplements: []Supplement{
					{ID: 202195, Content: "follow-up"},
				},
			},
		})
	})

	info, err := NewTopic(client).Get(1)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if path != "/v2/topics/1" {
		t.Errorf("path = %q, want /v2/topics/1", path)
	}
	if info.ID != 1 || info.Title != "Introducing Project Babel 2.0" {
		t.Errorf("unexpected topic: %+v", info)
	}
	if info.Member == nil || info.Member.Username != "Livid" {
		t.Errorf("unexpected member: %+v", info.Member)
	}
	if info.Node == nil || info.Node.Name != "babel" {
		t.Errorf("unexpected node: %+v", info.Node)
	}
	if len(info.Supplements) != 1 || info.Supplements[0].ID != 202195 {
		t.Errorf("unexpected supplements: %+v", info.Supplements)
	}
}

func TestTopicGetInvalidID(t *testing.T) {
	if _, err := NewTopic(&Client{}).Get(0); err == nil {
		t.Fatal("expected an error for a non-positive topic id")
	}
}

func TestTopicReplies(t *testing.T) {
	var path, page string
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		page = r.URL.Query().Get("p")
		writeJSON(t, w, http.StatusOK, topicRepliesResponse{
			v2Response: v2Response{Success: true, Message: "Topic replies found"},
			Result: []ReplyInfo{
				{
					ID:      1,
					Content: "nice work~",
					Member:  &MemberInfo{ID: 4, Username: "Jay"},
				},
			},
			Pagination: Pagination{PerPage: 20, Total: 178, Pages: 9},
		})
	})

	replies, pagination, err := NewTopic(client).Replies(1, 2)
	if err != nil {
		t.Fatalf("Replies: %v", err)
	}
	if path != "/v2/topics/1/replies" {
		t.Errorf("path = %q, want /v2/topics/1/replies", path)
	}
	if page != "2" {
		t.Errorf("p = %q, want 2", page)
	}
	if len(replies) != 1 || replies[0].ID != 1 {
		t.Errorf("unexpected replies: %+v", replies)
	}
	if replies[0].Member == nil || replies[0].Member.Username != "Jay" {
		t.Errorf("unexpected reply member: %+v", replies[0].Member)
	}
	if pagination.PerPage != 20 || pagination.Total != 178 || pagination.Pages != 9 {
		t.Errorf("unexpected pagination: %+v", pagination)
	}
}

func TestTopicRepliesInvalidID(t *testing.T) {
	if _, _, err := NewTopic(&Client{}).Replies(0, 1); err == nil {
		t.Fatal("expected an error for a non-positive topic id")
	}
}

func TestTopicLatest(t *testing.T) {
	var path string
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		writeJSON(t, w, http.StatusOK, []TopicInfo{
			{
				ID:      1,
				Title:   "first",
				Replies: 12,
				Member:  &MemberInfo{ID: 1, Username: "Livid"},
				Node:    &NodeInfo{ID: 90, Name: "python", Title: "Python"},
			},
			{ID: 2, Title: "second"},
		})
	})

	topics, err := NewTopic(client).Latest()
	if err != nil {
		t.Fatalf("Latest: %v", err)
	}
	if path != "/topics/latest.json" {
		t.Errorf("path = %q, want /topics/latest.json", path)
	}
	if len(topics) != 2 || topics[0].ID != 1 || topics[1].Title != "second" {
		t.Errorf("unexpected topics: %+v", topics)
	}
	if topics[0].Member == nil || topics[0].Member.Username != "Livid" {
		t.Errorf("unexpected member: %+v", topics[0].Member)
	}
	if topics[0].Node == nil || topics[0].Node.Name != "python" {
		t.Errorf("unexpected node: %+v", topics[0].Node)
	}
}

func TestTopicLatestAPIError(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "unavailable", http.StatusInternalServerError)
	})

	_, err := NewTopic(client).Latest()
	if err == nil || !strings.Contains(err.Error(), "500") {
		t.Fatalf("err = %v, want an error with status 500", err)
	}
}

func TestTopicHot(t *testing.T) {
	var path string
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		writeJSON(t, w, http.StatusOK, []TopicInfo{
			{
				ID:      10,
				Title:   "hot one",
				Replies: 30,
				Member:  &MemberInfo{ID: 2, Username: "Jay"},
				Node:    &NodeInfo{ID: 22, Name: "go", Title: "Go"},
			},
			{ID: 11, Title: "hot two"},
		})
	})

	topics, err := NewTopic(client).Hot()
	if err != nil {
		t.Fatalf("Hot: %v", err)
	}
	if path != "/topics/hot.json" {
		t.Errorf("path = %q, want /topics/hot.json", path)
	}
	if len(topics) != 2 || topics[0].ID != 10 || topics[1].Title != "hot two" {
		t.Errorf("unexpected topics: %+v", topics)
	}
	if topics[0].Member == nil || topics[0].Member.Username != "Jay" {
		t.Errorf("unexpected member: %+v", topics[0].Member)
	}
	if topics[0].Node == nil || topics[0].Node.Name != "go" {
		t.Errorf("unexpected node: %+v", topics[0].Node)
	}
}

func TestTopicHotAPIError(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "unavailable", http.StatusInternalServerError)
	})

	_, err := NewTopic(client).Hot()
	if err == nil || !strings.Contains(err.Error(), "500") {
		t.Fatalf("err = %v, want an error with status 500", err)
	}
}

// TestMemberAvatarURL covers both avatar shapes: the legacy API 1.0 sizes
// and the single avatar of API 2.0.
func TestMemberAvatarURL(t *testing.T) {
	legacy := MemberInfo{
		AvatarMini:   "mini.png",
		AvatarNormal: "normal.png",
		AvatarLarge:  "large.png",
	}
	if got := legacy.AvatarURL(); got != "normal.png" {
		t.Errorf("legacy AvatarURL() = %q, want normal.png", got)
	}

	var v2 MemberInfo
	if err := json.Unmarshal([]byte(`{"avatar":"large.png"}`), &v2); err != nil {
		t.Fatalf("unmarshal API 2.0 member: %v", err)
	}
	if got := v2.AvatarURL(); got != "large.png" {
		t.Errorf("API 2.0 AvatarURL() = %q, want large.png", got)
	}

	var nilMember *MemberInfo
	if got := nilMember.AvatarURL(); got != "" {
		t.Errorf("nil AvatarURL() = %q, want empty", got)
	}
}

// nodePageFixture is a trimmed node page: one row with a poster picture and
// one without, as a deleted member's row renders.
const nodePageFixture = `<html><body><div id="TopicsNode">
<div class="cell from_18392 t_1254691">
<table><tr><td width="48" valign="top" align="center"><a href="/member/revival83"><img src="https://cdn.v2ex.com/avatar/2b48/30a2/18392_xlarge.png?m=1755746557" class="avatar" border="0" align="default" width="48" alt="revival83" data-uid="18392" /></a></td></tr></table>
</div>
<div class="cell from_47049 t_1254601">
<table><tr><td width="48" valign="top" align="center"></td></tr></table>
</div>
</div></body></html>`

func TestTopicNodePagePosters(t *testing.T) {
	var path, page string
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		page = r.URL.Query().Get("p")
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = io.WriteString(w, nodePageFixture)
	})

	posters, err := NewTopic(client).NodePagePosters("qna", 2)
	if err != nil {
		t.Fatalf("NodePagePosters: %v", err)
	}
	if path != "/go/qna" {
		t.Errorf("path = %q, want /go/qna", path)
	}
	if page != "2" {
		t.Errorf("p = %q, want 2", page)
	}
	if len(posters) != 1 {
		t.Errorf("got %d posters, want 1: %+v", len(posters), posters)
	}
	poster := posters[1254691]
	if poster == nil {
		t.Fatalf("missing poster for 1254691: %+v", posters)
	}
	if poster.Username != "revival83" {
		t.Errorf("username = %q, want revival83", poster.Username)
	}
	if want := "https://cdn.v2ex.com/avatar/2b48/30a2/18392_xlarge.png?m=1755746557"; poster.AvatarURL() != want {
		t.Errorf("avatar = %q, want %q", poster.AvatarURL(), want)
	}
}

func TestTopicNodePagePostersEmptyName(t *testing.T) {
	if _, err := NewTopic(&Client{}).NodePagePosters("", 1); err == nil {
		t.Fatal("expected an error for an empty node name")
	}
}
