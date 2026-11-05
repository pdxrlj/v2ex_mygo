package components

import (
	"testing"

	"mygo_demo/internal/api"
)

// TestThreadRepliesBuildsConversations checks that a reply joins the comment
// it answers and that the untouched replies stay at the top level.
func TestThreadRepliesBuildsConversations(t *testing.T) {
	replies := []api.ReplyInfo{
		{ID: 1, Content: "first", Member: &api.MemberInfo{Username: "alpha"}},
		{ID: 2, Content: "@alpha thanks", Member: &api.MemberInfo{Username: "beta"}},
		{ID: 3, Content: "on its own", Member: &api.MemberInfo{Username: "gamma"}},
		{ID: 4, Content: "@beta hi", Member: &api.MemberInfo{Username: "alpha"}},
		// A self-mention is no reply target: it must keep the text as written.
		{ID: 5, Content: "@alpha adding a note", Member: &api.MemberInfo{Username: "alpha"}},
		// A reply that is only a mention keeps its words rather than an
		// empty body.
		{ID: 6, Content: "@gamma", Member: &api.MemberInfo{Username: "beta"}},
	}

	nodes := threadReplies(replies)
	if len(nodes) != len(replies) {
		t.Fatalf("got %d nodes, want %d", len(nodes), len(replies))
	}

	if nodes[0].parent != -1 {
		t.Errorf("first reply should start a conversation, parent = %d", nodes[0].parent)
	}
	if nodes[1].parent != 0 || nodes[1].replyFor != 1 {
		t.Errorf("reply 2 should answer reply 1: parent = %d, replyFor = %d",
			nodes[1].parent, nodes[1].replyFor)
	}
	if nodes[1].content != "thanks" {
		t.Errorf("mention should be split off, content = %q", nodes[1].content)
	}
	if nodes[2].parent != -1 {
		t.Errorf("reply 3 has no answer target, parent = %d", nodes[2].parent)
	}
	if nodes[3].parent != 1 || nodes[3].replyFor != 2 {
		t.Errorf("reply 4 should answer reply 2: parent = %d, replyFor = %d",
			nodes[3].parent, nodes[3].replyFor)
	}

	if len(nodes[0].children) != 1 || nodes[0].children[0] != 1 {
		t.Errorf("reply 1 children = %v, want [1]", nodes[0].children)
	}
	if len(nodes[1].children) != 1 || nodes[1].children[0] != 3 {
		t.Errorf("reply 2 children = %v, want [3]", nodes[1].children)
	}

	var roots []int
	for i := range nodes {
		if nodes[i].parent < 0 {
			roots = append(roots, i)
		}
	}
	if len(roots) != 3 || roots[0] != 0 || roots[1] != 2 || roots[2] != 4 {
		t.Errorf("roots = %v, want [0 2 4]", roots)
	}

	if nodes[4].replyTo != "" || nodes[4].content != "@alpha adding a note" {
		t.Errorf("self-mention should keep text and no target: replyTo = %q, content = %q",
			nodes[4].replyTo, nodes[4].content)
	}
	if nodes[5].replyTo != "gamma" || nodes[5].content != "@gamma" || nodes[5].parent != 2 {
		t.Errorf("mention-only reply should keep its words and thread: replyTo = %q, content = %q, parent = %d",
			nodes[5].replyTo, nodes[5].content, nodes[5].parent)
	}
}
