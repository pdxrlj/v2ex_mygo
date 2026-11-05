package components

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"

	"mygo_demo/assets/img"
	"mygo_demo/internal/api"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown"
)

// topicWindow is the single topic window the app keeps open. Opening a
// topic while one is shown closes it first, so the detail never stacks
// windows. The reference also keeps the open window alive.
var topicWindow *mygo.Window

// OpenTopicWindow opens a window showing one topic: its body on top, fixed,
// and every reply below it, scrolling in the space left.
func OpenTopicWindow(client *api.Client, proxy string, topic api.TopicInfo) {
	if topicWindow != nil {
		topicWindow.Close()
		topicWindow = nil
	}

	tw := &TopicWindow{
		client:    client,
		proxy:     proxy,
		topicID:   topic.ID,
		repliesMD: map[int]*markdown.Options{},
	}

	tw.win = mygo.NewWindow(mygo.WindowOptions{
		Title:           topic.Title,
		Width:           760,
		Height:          700,
		MinWidth:        560,
		MinHeight:       420,
		BackgroundColor: "light-dark(#f5f6f8, #16181c)",
		Content:         ui.View(tw.Draw),
	})
	topicWindow = tw.win
	// Closing the window by hand drops the reference, so the next topic
	// opens a fresh one. The check keeps a replaced window's close event
	// from clearing its successor.
	tw.win.OnClosed(func() {
		if topicWindow == tw.win {
			topicWindow = nil
		}
	})
	if err := tw.win.SetIcon(img.LogoPNG); err != nil {
		log.Printf("topic window icon: %v", err)
	}
}

// TopicWindow is the state of one topic window: the topic, the replies
// loaded so far and where the reads stand.
type TopicWindow struct {
	client  *api.Client
	proxy   string
	topicID int

	win *mygo.Window

	topic  *api.TopicInfo
	err    error
	loaded bool

	// mu guards replies, replyIDs and loading, which a background reader
	// fills while the window draws.
	mu       sync.Mutex
	replies  []api.ReplyInfo
	replyIDs map[int]struct{} // ids already in replies, to drop page overlaps
	loading  bool

	// body holds the topic's Markdown; repliesMD holds one Options per
	// reply id, so each keeps its parse between frames.
	body      markdown.Options
	repliesMD map[int]*markdown.Options
}

func (t *TopicWindow) invalidate() {
	if t.win != nil {
		t.win.Invalidate()
	}
}

func (t *TopicWindow) Draw(c *ui.Context) {
	t.load()

	th := c.Theme()
	// The whole page scrolls as one: a topic body can be longer than the
	// window, and a body that held its size would push the replies out of
	// sight.
	ui.Scroll(c).Fill().Children(func() {
		ui.Column(c).FillWidth().Padding(24).Gap(16).Children(func() {
			switch {
			case t.err != nil:
				ui.Text(c, "加载主题失败："+t.err.Error()).
					FontSize(13).TextColor(th.TextMuted)
			case t.topic == nil:
				ui.Text(c, "正在加载主题…").FontSize(13).TextColor(th.TextMuted)
			default:
				t.header(c)
				ui.Box(c).FillWidth().Height(1).Background(th.Border.Alpha(0.25))
				markdown.View(c, t.topic.Content, &t.body)
				t.repliesSection(c)
			}
		})
	})
}

// load fetches the topic and its first page of replies; the pages after are
// read in the background so the window shows what it has meanwhile.
func (t *TopicWindow) load() {
	if t.loaded {
		return
	}
	t.loaded = true

	topic, err := api.NewTopic(t.client).Get(t.topicID)
	if err != nil {
		t.err = err
		return
	}
	t.topic = topic

	replies, pagination, err := api.NewTopic(t.client).Replies(t.topicID, 1)
	if err != nil {
		t.err = err
		return
	}

	t.mu.Lock()
	t.replies = replies
	t.replyIDs = make(map[int]struct{}, len(replies))
	for _, r := range replies {
		t.replyIDs[r.ID] = struct{}{}
	}
	t.loading = pagination.Pages > 1
	t.mu.Unlock()

	if pagination.Pages > 1 {
		go t.loadRemaining(pagination.Pages)
	}
}

// loadRemaining reads pages 2 to last, appending each as it arrives. A
// reply posted while the pages are being read shifts what a later page
// holds, so an id already loaded is dropped rather than drawn twice.
func (t *TopicWindow) loadRemaining(pages int) {
	defer func() {
		t.mu.Lock()
		t.loading = false
		t.mu.Unlock()
		t.invalidate()
	}()

	for page := 2; page <= pages; page++ {
		replies, _, err := api.NewTopic(t.client).Replies(t.topicID, page)
		if err != nil {
			return
		}

		t.mu.Lock()
		for _, r := range replies {
			if _, ok := t.replyIDs[r.ID]; ok {
				continue
			}
			t.replyIDs[r.ID] = struct{}{}
			t.replies = append(t.replies, r)
		}
		t.mu.Unlock()
		t.invalidate()
	}
}

// header shows where the topic lives, its title and who wrote it.
func (t *TopicWindow) header(c *ui.Context) {
	th := c.Theme()
	topic := t.topic

	node := "节点主题"
	if topic.Node != nil && topic.Node.Title != "" {
		node = topic.Node.Title + " / 节点主题"
	}

	author := "匿名用户"
	avatarURL := ""
	if topic.Member != nil {
		if topic.Member.Username != "" {
			author = topic.Member.Username
		}
		avatarURL = topic.Member.AvatarURL()
	}

	ui.Column(c).FillWidth().Gap(8).Children(func() {
		ui.Row(c).FillWidth().AlignItems(ui.Center).Children(func() {
			ui.Row(c).Gap(5).AlignItems(ui.Center).Children(func() {
				ui.Text(c, node).FontSize(11).TextColor(th.TextMuted)
				ui.Text(c, "›").FontSize(11).TextColor(th.TextMuted)
				ui.Text(c, "主题详情").FontSize(11).TextColor(th.Accent)
			})
			ui.Spacer(c)
			ui.Text(c, "#"+strconv.Itoa(topic.ID)).FontSize(11).TextColor(th.TextMuted)
		})

		ui.Text(c, topic.Title).FontSize(24).FontWeight(700).TextColor(th.Text).MaxLines(2)

		ui.Row(c).FillWidth().Gap(12).AlignItems(ui.Center).Children(func() {
			ui.Avatar(c, author, avatarBitmap(t.proxy, avatarURL, t.invalidate)).Size(36, 36)

			ui.Column(c).Grow(1).Gap(3).Children(func() {
				ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
					ui.Text(c, author).FontSize(12).FontWeight(600).TextColor(th.Text)
					if topic.Member != nil && topic.Member.Pro > 0 {
						ui.Badge(c, "PRO")
					}
				})
				meta := formatDateTime(topic.Created) +
					fmt.Sprintf(" · %d 回复 · %d 收藏 · %d 感谢",
						topic.Replies, topic.Stars, topic.Thanks)
				ui.Text(c, meta).FontSize(11).TextColor(th.TextMuted)
			})

			ui.PrimaryButton(c, "在浏览器打开").OnClick(func() {
				go mygo.Shell.OpenExternal(topicURL(*topic))
			})
		})
	})
}

// repliesSection fills the room under the body as part of the page's one
// scroll, as a continuous thread instead of a stack of cards, so the list
// reads like the post streams of Discourse and Flarum.
func (t *TopicWindow) repliesSection(c *ui.Context) {
	th := c.Theme()

	t.mu.Lock()
	replies := append([]api.ReplyInfo(nil), t.replies...)
	loading := t.loading
	t.mu.Unlock()

	ui.Column(c).FillWidth().Gap(10).Children(func() {
		ui.Row(c).FillWidth().AlignItems(ui.Center).Children(func() {
			ui.Text(c, fmt.Sprintf("全部回复（%d）", t.topic.Replies)).
				FontSize(14).FontWeight(600).TextColor(th.Text).Grow(1)
			if loading {
				ui.Text(c, fmt.Sprintf("加载中 %d/%d", len(replies), t.topic.Replies)).
					FontSize(11).TextColor(th.TextMuted)
			} else {
				ui.Text(c, fmt.Sprintf("已加载 %d 条", len(replies))).
					FontSize(11).TextColor(th.TextMuted)
			}
		})

		ui.Box(c).FillWidth().Height(1).Background(th.Border.Alpha(0.2))

		if len(replies) == 0 {
			ui.Text(c, "还没有回复。").FontSize(12).TextColor(th.TextMuted)
			return
		}

		ui.Column(c).FillWidth().Padding(0, 0, 8, 0).Children(func() {
			nodes := threadReplies(replies)
			roots := 0
			for i := range nodes {
				if nodes[i].parent >= 0 {
					continue
				}
				if roots > 0 {
					ui.Box(c).FillWidth().Height(1).
						Background(th.Border.Alpha(0.2))
				}
				t.replyThread(c, nodes, i, 0, false)
				roots++
			}
		})
	})
}

// replyThread draws one reply and, below it, the answers it got: the group
// is indented and joined by a rail, so a conversation stays together and
// its parent is always in view, as Reddit and Hacker News do. Answers
// deeper than maxReplyDepth keep the last indent to leave the text room,
// and carry the reply-target tag, as their parent is no longer the row
// right above them.
func (t *TopicWindow) replyThread(c *ui.Context, nodes []replyNode, index, depth int, attached bool) {
	node := nodes[index]
	t.replyRow(c, node, attached)

	if len(node.children) == 0 {
		return
	}
	if depth >= maxReplyDepth {
		for _, child := range node.children {
			t.replyThread(c, nodes, child, depth, false)
		}
		return
	}

	th := c.Theme()
	group := ui.Column(c).FillWidth().
		Padding(0, 0, 0, 16).
		Border(2, th.Border.Alpha(0.25)).
		BorderWidth(0, 0, 0, 2)
	group.Children(func() {
		for _, child := range node.children {
			t.replyThread(c, nodes, child, depth+1, true)
		}
	})
}

// replyNode is one reply in the thread: which earlier reply it answers,
// and the text with its @mention split off. A node whose parent is -1 starts
// a conversation; the others hang under the reply they answer.
type replyNode struct {
	reply    api.ReplyInfo
	content  string
	floor    int
	replyTo  string
	replyFor int
	parent   int
	children []int
}

// maxReplyDepth is how many levels a nested conversation is indented; a
// deeper answer keeps the last level's indent so the text stays readable.
const maxReplyDepth = 3

// threadReplies turns the flat reply list into conversations: each reply
// joins the earlier reply it answers, so a parent is always drawn just
// above its answers, and the floors stay ordered inside every branch.
func threadReplies(replies []api.ReplyInfo) []replyNode {
	lastFloor := map[string]int{}
	nodes := make([]replyNode, 0, len(replies))

	for i, reply := range replies {
		author := ""
		if reply.Member != nil {
			author = reply.Member.Username
		}

		mention, body := splitMention(reply.Content)

		node := replyNode{
			reply:   reply,
			content: reply.Content,
			floor:   i + 1,
			parent:  -1,
		}
		if mention != "" && mention != author {
			// The mention becomes the row's reply-target tag, so it comes
			// off the text. A self-mention is no reply target: it keeps the
			// text as written, or the words would vanish with no indicator.
			node.content = body
			if node.content == "" {
				node.content = reply.Content
			}
			node.replyTo = mention
			if target, ok := lastFloor[mention]; ok {
				node.replyFor = target + 1
				node.parent = target
			}
		}

		index := len(nodes)
		nodes = append(nodes, node)
		if node.parent >= 0 {
			nodes[node.parent].children = append(nodes[node.parent].children, index)
		}
		if author != "" {
			lastFloor[author] = index
		}
	}
	return nodes
}

// splitMention takes a leading @name off a reply, as V2EX writes the person
// a reply answers that way. Anything that is not a plain ASCII username is
// left in the text, so a sentence starting with "@某人" keeps its words.
func splitMention(content string) (mention, rest string) {
	s := strings.TrimSpace(content)
	if !strings.HasPrefix(s, "@") {
		return "", s
	}

	end := 1
	for end < len(s) {
		r := s[end]
		switch {
		case r == ' ' || r == '\t' || r == '\r' || r == '\n':
			if end == 1 {
				return "", s
			}
			return s[1:end], strings.TrimSpace(s[end:])
		case r == '-' || r == '_':
		case r >= '0' && r <= '9':
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z':
		default:
			// Not a username: keep the whole sentence.
			return "", s
		}
		end++
	}

	return s[1:], ""
}

// replyRow draws one reply as a row of the thread: avatar in a narrow
// gutter, meta and content beside it, no card of its own. The meta line
// carries the author, the poster and reply-target tags and, at the right,
// the floor and the time; the content starts under the author, not under
// the avatar, so the eye follows one column down the thread. The target
// tag is drawn only when the parent is not the row right above: a root,
// or a reply past the nesting cap.
func (t *TopicWindow) replyRow(c *ui.Context, node replyNode, attached bool) {
	th := c.Theme()
	reply := node.reply

	author := "匿名用户"
	avatarURL := ""
	if reply.Member != nil {
		if reply.Member.Username != "" {
			author = reply.Member.Username
		}
		avatarURL = reply.Member.AvatarURL()
	}

	isPoster := t.topic != nil && t.topic.Member != nil &&
		t.topic.Member.Username != "" && author == t.topic.Member.Username

	md, ok := t.repliesMD[reply.ID]
	if !ok {
		md = &markdown.Options{}
		t.repliesMD[reply.ID] = md
	}

	row := ui.Row(c).FillWidth().
		Padding(10, 8).
		Radius(8).
		Gap(12).
		AlignItems(ui.Start)
	// Rows have no fill of their own; a light one follows the pointer so
	// a long thread still shows which reply is under it.
	if row.Hovered() {
		row.Background(th.Surface)
	}

	row.Children(func() {
		ui.Avatar(c, author,
			avatarBitmap(t.proxy, avatarURL, t.invalidate)).Size(26, 26)

		ui.Column(c).Grow(1).Gap(4).Children(func() {
			ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(6).Children(func() {
				ui.Text(c, author).FontSize(12).FontWeight(600).TextColor(th.Text)
				if isPoster {
					t.replyTag(c, "楼主", th.Accent, th.Accent.Alpha(0.12))
				}
				if node.replyTo != "" && (node.parent < 0 || !attached) {
					target := "回复 @" + node.replyTo
					if node.replyFor > 0 {
						target += " · #" + strconv.Itoa(node.replyFor)
					}
					t.replyTag(c, target, th.Accent, th.Accent.Alpha(0.08))
				}
				ui.Spacer(c)
				ui.Text(c, fmt.Sprintf("#%d · %s",
					node.floor, timeAgo(reply.Created))).
					FontSize(10).TextColor(th.TextMuted)
			})

			markdown.View(c, node.content, md)
		})
	})
}

// replyTag draws a small pill in a reply's meta line, as the poster badge
// and the reply target use.
func (t *TopicWindow) replyTag(c *ui.Context, label string, fg, bg ui.Color) {
	ui.Box(c).Padding(1, 6).Radius(4).Background(bg).Children(func() {
		ui.Text(c, label).FontSize(10).TextColor(fg).SingleLine()
	})
}
