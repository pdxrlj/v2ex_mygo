package components

import (
	"errors"
	"fmt"
	"log"
	"net/url"
	"path"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"mygo_demo/assets/img"
	"mygo_demo/internal/api"
	"mygo_demo/internal/database/repo"
	"mygo_demo/internal/ui/components/icons"

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
func OpenTopicWindow(v *View, topic api.TopicInfo) {
	if topicWindow != nil {
		topicWindow.Close()
		topicWindow = nil
	}

	tw := &TopicWindow{
		client:       v._client,
		proxy:        v.ProxyURL,
		owner:        v,
		topicID:      topic.ID,
		repliesMD:    map[int]*markdown.Options{},
		splitPercent: loadTopicSplit(v.SettingsRepo),
		// The body's pictures load through the app's proxy too.
		body: markdown.Options{LoadImage: markdownImages(v.ProxyURL)},
	}

	tw.win = mygo.NewWindow(mygo.WindowOptions{
		Title:  topic.Title,
		Width:  980,
		Height: 760,

		MinWidth:          820,
		MinHeight:         520,
		BackgroundColor:   "light-dark(#f5f6f8, #16181c)",
		Content:           ui.View(tw.Draw),
		TitleBarStyle:     mygo.TitleBarHidden,
		HideWindowButtons: true,
	})
	topicWindow = tw.win
	// Closing the window by hand drops the reference, so the next topic
	// opens a fresh one. The check keeps a replaced window's close event
	// from clearing its successor.
	tw.win.OnClosed(func() {
		if topicWindow == tw.win {
			topicWindow = nil
		}
		if tw.splitDirty {
			tw.saveSplit()
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
	// owner is the view whose subscription controls the window shows.
	owner *View

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
	// reply id, so each keeps its parse between frames. bodyScroll and
	// replyScroll keep where each pane stands while the other scrolls.
	body      markdown.Options
	repliesMD map[int]*markdown.Options

	bodyScroll  ui.ScrollState
	replyScroll ui.ScrollState

	// replyDraft is the text the composer edits, on the UI thread.
	replyDraft string
	// replySending, replyNotice, replyNoticeOK and replySent are the
	// composer's state, which mu guards while the post runs; pendingTopic is
	// a reloaded topic waiting for the UI thread to take it.
	replySending  bool
	replyNotice   string
	replyNoticeOK bool
	replySent     bool
	pendingTopic  *api.TopicInfo

	// splitPercent is the topic pane's share of the width, which the divider
	// moves; splitSaved throttles saving it while the user drags.
	splitPercent float32
	splitSaved   time.Time
	splitDirty   bool
	// splitDragging tracks a drag that began on the divider's handle, so the
	// position is saved once the pointer is released.
	splitDragging bool
}

func (t *TopicWindow) invalidate() {
	if t.win != nil {
		t.win.Invalidate()
	}
}

// invalidateTopicWindow redraws the open topic window, if any, so its
// subscription panel shows the state the poll just changed.
func invalidateTopicWindow() {
	if topicWindow != nil {
		topicWindow.Invalidate()
	}
}

func (t *TopicWindow) Draw(c *ui.Context) {
	t.load()
	if pending := t.takePendingTopic(); pending != nil {
		t.topic = pending
	}
	t.mu.Lock()
	if t.replySent {
		t.replyDraft = ""
		t.replySent = false
	}
	t.mu.Unlock()

	th := c.Theme()
	ui.Column(c).Fill().Background(th.Surface).Children(func() {
		t.nav(c)
		// The same hairline the main window puts under its header.
		ui.Box(c).FillWidth().Height(1).Background(th.Border.Alpha(0.2))

		switch {
		case t.err != nil:
			ui.Text(c, "加载主题失败："+t.err.Error()).
				FontSize(13).TextColor(th.TextMuted).Padding(24)
		case t.topic == nil:
			ui.Text(c, "正在加载主题…").
				FontSize(13).TextColor(th.TextMuted).Padding(24)
		default:
			// Two panes the divider resizes: the topic on the left and its
			// replies on the right. Each has a scroll of its own, so reading
			// replies never moves the topic away.
			// Grow, not Fill: the nav above already took its share, so the
			// panes take what is left rather than the whole window's height.
			ui.Column(c).FillWidth().Grow(1).Padding(24).Children(func() {
				ui.Box(c).Fill().Children(func() {
					split := ui.SplitPercent(c, &t.splitPercent,
						func() {
							ui.Column(c).FillHeight().Padding(0, 12, 0, 0).Gap(12).
								Children(func() {
									// The title and author stay in view while the
									// body scrolls under the hairline.
									t.header(c)
									ui.Box(c).FillWidth().Height(1).
										Background(th.Border.Alpha(0.25))

									ui.Scroll(c).FillWidth().Grow(1).
										TrackScroll(&t.bodyScroll).
										Children(func() {
											ui.Column(c).FillWidth().
												Children(func() {
													markdown.View(c, t.topic.Content, &t.body)
												})
										})

									t.replyComposer(c)
								})
						},
						func() {
							ui.Column(c).FillHeight().Padding(0, 0, 0, 12).
								Children(func() {
									// The section's title stays put while the
									// replies scroll under it.
									t.repliesHeader(c)

									ui.Scroll(c).FillWidth().Grow(1).
										TrackScroll(&t.replyScroll).
										Children(func() {
											// Leave room between the reply text and
											// the scroll bar, which the scroll draws
											// on its own edge.
											ui.Column(c).FillWidth().Padding(0, 16, 0, 0).
												Children(func() {
													t.repliesList(c)
												})
										})
								})
						},
					).Fill()

					if split.Changed() {
						t.splitDirty = true
						if time.Since(t.splitSaved) >= 300*time.Millisecond {
							t.saveSplit()
						}
					}

					t.splitHandle(c)
				})
			})
		}
	})
}

// topicNavHeight is the window's own title bar, which replaces the system's.
const topicNavHeight = 52

// nav draws the window's title bar, laid out as the main window's header:
// the logo, where the topic sits, the theme toggle and the window controls.
// The bar itself drags the window, as the system's title bar would.
func (t *TopicWindow) nav(c *ui.Context) {
	th := c.Theme()

	ui.Row(c).FillWidth().Height(topicNavHeight).Padding(0, 16).Gap(10).
		AlignItems(ui.Center).
		Background(th.Surface).
		DragWindow().
		Children(func() {
			ui.Image(c, img.Logo).Size(24, 24).Fit(ui.Contain)
			ui.Text(c, "V2EX").FontSize(14).FontWeight(700).TextColor(th.Text)

			ui.Row(c).Gap(6).AlignItems(ui.Center).MarginStart(12).
				Children(func() {
					ui.Text(c, "全部节点").FontSize(12).TextColor(th.TextMuted)
					ui.Text(c, "›").FontSize(12).TextColor(th.TextMuted)
					if t.topic != nil && t.topic.Node != nil &&
						t.topic.Node.Title != "" {
						ui.Text(c, t.topic.Node.Title).FontSize(12).
							TextColor(th.TextMuted)
						ui.Text(c, "›").FontSize(12).TextColor(th.TextMuted)
					}
					ui.Text(c, "主题详情").FontSize(12).FontWeight(600).
						TextColor(th.Text)
					if t.topic != nil {
						ui.Text(c, "#"+strconv.Itoa(t.topic.ID)).
							FontSize(12).TextColor(th.TextMuted)
					}
				})

			ui.Spacer(c)

			// The detail lives in a window of its own, so the way back to the
			// main one raises it and leaves this window. The arrow matches the
			// header's other icon buttons.
			headerButton(c, icons.ArrowLeft, "返回主界面", func() {
				if t.owner != nil && t.owner.Win != nil {
					if t.owner.Win.IsMinimized() {
						t.owner.Win.Restore()
					}
					t.owner.Win.Show()
					t.owner.Win.Focus()
				}
				if t.win != nil {
					t.win.Close()
				}
			})

			headerButton(c, icons.Theme(mygo.Theme.IsDark()), "切换外观", func() {
				if mygo.Theme.IsDark() {
					mygo.Theme.SetSource(mygo.ThemeLight)
				} else {
					mygo.Theme.SetSource(mygo.ThemeDark)
				}
			})

			windowControls(c, t.win)
		})
}

// The divider defaults near the middle and stays wide enough on both sides
// to keep the panes readable.
const (
	defaultTopicSplit = 55
	minTopicSplit     = 20
	maxTopicSplit     = 80
)

// loadTopicSplit reads the saved divider position, as the topic pane's
// percentage of the width.
func loadTopicSplit(settings *repo.Settings) float32 {
	percent := float32(defaultTopicSplit)
	if raw, ok, err := settings.Get(repo.TopicSplitKey); err == nil && ok {
		if value, err := strconv.ParseFloat(raw, 32); err == nil {
			percent = float32(value)
		}
	}
	return min(max(percent, minTopicSplit), maxTopicSplit)
}

// saveSplit stores the divider position, throttled while a drag runs and
// again as the window closes.
func (t *TopicWindow) saveSplit() {
	if t.owner == nil {
		return
	}
	t.splitDirty = false
	t.splitSaved = time.Now()
	_ = t.owner.SettingsRepo.Set(repo.TopicSplitKey,
		strconv.FormatFloat(float64(t.splitPercent), 'f', 1, 32))
}

// splitHandle draws the faint split-view grip on the divider, which brightens
// under the pointer. The grip itself takes the drag, so the small area around
// it resizes the panes like the line does.
func (t *TopicWindow) splitHandle(c *ui.Context) {
	width, height := c.Size()
	// The grip sits over the split's box, which the window's nav, its
	// hairline and the content padding leave inset from the window's size.
	areaW := float32(width) - 48
	areaH := float32(height) - topicNavHeight - 49
	if areaW < 1 || areaH < 1 {
		return
	}

	x := (areaW - 1) * t.splitPercent / 100
	y := areaH / 2
	th := c.Theme()

	handle := ui.Box(c).Absolute().
		Left(x-10).Top(y-28).
		Size(20, 56).
		Radius(10).
		Center().
		Cursor(ui.CursorResizeColumn).
		Tooltip("拖动调整左右宽度")
	dx, _, held := handle.Dragged()
	hovered := handle.Hovered()

	if held {
		t.splitDragging = true
		// Dragged reports the pointer's move since the last frame, so the
		// percentage moves by that delta each frame, as the split's own
		// handle does.
		if dx != 0 && areaW > 1 {
			percent := t.splitPercent + dx*100/(areaW-1)
			percent = min(max(percent, minTopicSplit), maxTopicSplit)
			if percent != t.splitPercent {
				t.splitPercent = percent
				t.splitDirty = true
				if time.Since(t.splitSaved) >= 300*time.Millisecond {
					t.saveSplit()
				}
			}
		}
	} else if t.splitDragging {
		t.splitDragging = false
		if t.splitDirty {
			t.saveSplit()
		}
	}

	if hovered || held {
		handle.Background(th.Surface)
	}
	color := th.TextMuted.Alpha(0.35)
	if hovered || held {
		color = th.TextMuted.Alpha(0.85)
	}
	handle.Children(func() {
		ui.Icon(c, icons.SplitView).FontSize(18).TextColor(color)
	})
}

// takePendingTopic hands the topic a background reload made to the UI
// thread, which owns t.topic.
func (t *TopicWindow) takePendingTopic() *api.TopicInfo {
	t.mu.Lock()
	defer t.mu.Unlock()
	pending := t.pendingTopic
	t.pendingTopic = nil
	return pending
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

// header shows the topic's title and who wrote it; where the topic lives is
// in the window's nav above.
func (t *TopicWindow) header(c *ui.Context) {
	th := c.Theme()
	topic := t.topic

	author := "匿名用户"
	avatarURL := ""
	if topic.Member != nil {
		if topic.Member.Username != "" {
			author = topic.Member.Username
		}
		avatarURL = topic.Member.AvatarURL()
	}

	ui.Column(c).FillWidth().Gap(8).Children(func() {
		ui.Text(c, topic.Title).FontSize(24).FontWeight(700).TextColor(th.Text).MaxLines(3)

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

			t.subscriptionHeaderButton(c)

			ui.PrimaryButton(c, "在浏览器打开").OnClick(func() {
				go mygo.Shell.OpenExternal(topicURL(*topic))
			})
		})
	})
}

// replyComposer draws the reply box at the bottom of the topic pane. It
// posts through the site's form, so it asks for the web login until one is
// saved.
func (t *TopicWindow) replyComposer(c *ui.Context) {
	if t.owner == nil || t.topic == nil {
		return
	}
	th := c.Theme()
	cookie, _, _, _, _ := t.owner.webSnapshot()

	t.mu.Lock()
	sending := t.replySending
	notice := t.replyNotice
	noticeOK := t.replyNoticeOK
	t.mu.Unlock()

	ui.Column(c).FillWidth().Padding(12).Radius(10).
		Background(th.Surface).
		Border(1, th.Border.Alpha(0.4)).
		Gap(8).
		Children(func() {
			if cookie == "" {
				ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(8).Children(func() {
					ui.Text(c, "登录 V2EX 网页版后可以回复主题").
						FontSize(11).TextColor(th.TextMuted).Grow(1)
					ui.ButtonBase(c).Label("网页登录").
						Padding(4, 8).Radius(6).
						Background(th.Background).
						Border(1, th.Border.Alpha(0.4)).
						Children(func() {
							ui.Text(c, "网页登录").FontSize(11).TextColor(th.Text)
						}).
						OnClick(func() {
							proxy := t.owner.ProxyURL
							OpenWebLoginWindow(func(cookie string) {
								t.owner.onWebLogin(cookie, proxy)
								invalidateTopicWindow()
							})
						})
				})
				return
			}

			ui.Text(c, "回复主题").FontSize(11).TextColor(th.TextMuted)
			ui.TextArea(c, &t.replyDraft).
				FontSize(12).
				Height(64).
				Padding(6, 8).
				Placeholder("写下你的回复…").
				Label("回复内容")
			ui.Text(c, "评论不支持 Markdown；换行、链接、@ 和图片链接会正常显示。").
				FontSize(10).TextColor(th.TextMuted)

			ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(8).Children(func() {
				if notice != "" {
					color := th.Accent
					if !noticeOK {
						color = th.Warning
					}
					ui.Text(c, notice).FontSize(11).TextColor(color).
						Grow(1).MaxLines(2)
				} else {
					ui.Spacer(c)
				}

				label := "发表评论"
				if sending {
					label = "发送中…"
				}
				ui.ButtonBase(c).Label("发表评论").
					Padding(4, 10).Radius(6).Background(th.Accent).
					Children(func() {
						ui.Text(c, label).FontSize(11).
							TextColor(ui.Hex("#FFFFFF"))
					}).
					OnClick(func() {
						t.submitReply()
					})
			})
		})
}

// submitReply posts the draft through the web session and reloads the
// replies on success.
func (t *TopicWindow) submitReply() {
	if t.owner == nil || t.topic == nil {
		return
	}
	content := strings.TrimSpace(t.replyDraft)
	if content == "" {
		t.setReplyNotice("回复内容不能为空", false)
		return
	}
	cookie, _, _, _, _ := t.owner.webSnapshot()
	if cookie == "" {
		t.setReplyNotice("请先完成网页登录", false)
		return
	}

	t.mu.Lock()
	if t.replySending {
		t.mu.Unlock()
		return
	}
	t.replySending = true
	t.replyNotice = ""
	t.mu.Unlock()

	proxy := t.owner.ProxyURL
	topicID := t.topic.ID
	t.invalidate()

	go func() {
		session, err := api.NewWebSession(proxy, cookie)
		if err == nil {
			err = session.Reply(topicID, content)
		}

		t.mu.Lock()
		t.replySending = false
		if err != nil {
			t.replyNotice = "评论失败：" + err.Error()
			t.replyNoticeOK = false
		} else {
			t.replyNotice = "评论成功"
			t.replyNoticeOK = true
			t.replySent = true
		}
		t.mu.Unlock()

		if errors.Is(err, api.ErrWebSignedOut) {
			t.owner.forgetWebCookie()
			t.setReplyNotice("登录已失效，请重新登录", false)
			return
		}
		if err == nil {
			t.reloadReplies()
		}
		t.invalidate()
	}()
}

// setReplyNotice reports the composer's outcome.
func (t *TopicWindow) setReplyNotice(msg string, ok bool) {
	t.mu.Lock()
	t.replyNotice = msg
	t.replyNoticeOK = ok
	t.mu.Unlock()
	t.invalidate()
}

// reloadReplies rereads the topic and its replies after a post, so the new
// one shows and the count moves.
func (t *TopicWindow) reloadReplies() {
	topic, err := api.NewTopic(t.client).Get(t.topicID)
	if err != nil {
		return
	}
	replies, pagination, err := api.NewTopic(t.client).Replies(t.topicID, 1)
	if err != nil {
		return
	}

	t.mu.Lock()
	t.pendingTopic = topic
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
	t.invalidate()
}

// repliesHeader is the panel's fixed heading: the reply count and how many
// are loaded, over a hairline, so they stay in view while the replies
// scroll under them.
func (t *TopicWindow) repliesHeader(c *ui.Context) {
	th := c.Theme()

	t.mu.Lock()
	loaded := len(t.replies)
	loading := t.loading
	t.mu.Unlock()

	ui.Column(c).FillWidth().Padding(0, 16, 10, 0).Gap(8).Children(func() {
		ui.Row(c).FillWidth().AlignItems(ui.Center).Children(func() {
			ui.Text(c, fmt.Sprintf("全部回复（%d）", t.topic.Replies)).
				FontSize(14).FontWeight(600).TextColor(th.Text).Grow(1)
			if loading {
				ui.Text(c, fmt.Sprintf("加载中 %d/%d", loaded, t.topic.Replies)).
					FontSize(11).TextColor(th.TextMuted)
			} else {
				ui.Text(c, fmt.Sprintf("已加载 %d 条", loaded)).
					FontSize(11).TextColor(th.TextMuted)
			}
		})

		ui.Box(c).FillWidth().Height(1).Background(th.Border.Alpha(0.2))
	})
}

// repliesList draws the replies as a continuous thread instead of a stack of
// cards, so the list reads like the post streams of Discourse and Flarum.
func (t *TopicWindow) repliesList(c *ui.Context) {
	th := c.Theme()

	t.mu.Lock()
	replies := append([]api.ReplyInfo(nil), t.replies...)
	t.mu.Unlock()

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

// maxReplyPictureHeight is the tallest a reply's picture shows in the pane.
// A smaller picture keeps its own size; a taller one becomes a thumbnail,
// which shows its original in the browser when clicked.
const maxReplyPictureHeight = 260

// maxReplyEmoticonHeight is the size an emoji or sticker shows at in a
// reply: beside the words, not across the pane.
const maxReplyEmoticonHeight = 28

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
		md = &markdown.Options{LoadImage: markdownImages(t.proxy)}
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

			t.replyContent(c, node.content, md)
		})
	})
}

// pictureURL matches a bare http(s) address in a reply. Whether it points at
// a picture is checked after the match.
var pictureURL = regexp.MustCompile(`https?://[^\s<>"']+`)

// pictureLink is one bare picture address in a reply's text, with where it
// sits in that text.
type pictureLink struct {
	url        string
	start, end int
}

// pictureLinks finds the bare addresses in content that name a picture file.
// Each is loaded in the background; replyContent shows the ones that decode
// as pictures and leaves the rest in the text.
func pictureLinks(content string) []pictureLink {
	var links []pictureLink
	for _, m := range pictureURL.FindAllStringIndex(content, -1) {
		// A destination of markdown's own link or image syntax is already
		// drawn by the markdown view.
		if m[0] >= 2 && content[m[0]-2:m[0]] == "](" {
			continue
		}

		raw := strings.TrimRight(content[m[0]:m[1]],
			".,;:!?)]}>，。；：！？、）】》」』…")
		if !isPictureAddress(raw) {
			continue
		}
		links = append(links, pictureLink{url: raw, start: m[0], end: m[0] + len(raw)})
	}
	return links
}

// isPictureAddress reports whether link names a picture file.
func isPictureAddress(link string) bool {
	u, err := url.Parse(link)
	if err != nil {
		return false
	}
	switch strings.ToLower(path.Ext(u.Path)) {
	case ".png", ".jpg", ".jpeg", ".gif", ".webp", ".bmp", ".svg":
		return true
	}
	return false
}

// inlinePictureLink reports whether the link shares its line with words, as
// an emoji used inside a sentence does, rather than standing alone as a
// posted picture does.
func inlinePictureLink(content string, link pictureLink) bool {
	lineStart := strings.LastIndexByte(content[:link.start], '\n') + 1
	lineEnd := len(content)
	if i := strings.IndexByte(content[link.end:], '\n'); i >= 0 {
		lineEnd = link.end + i
	}
	return strings.TrimSpace(content[lineStart:link.start]) != "" ||
		strings.TrimSpace(content[link.end:lineEnd]) != ""
}

// squarePicture reports whether the picture is about as wide as it is high,
// as emojis and stickers are and wide screenshots are not.
func squarePicture(img ui.ImageSource) bool {
	w, h := ui.ImageSize(img)
	if h <= 0 {
		return false
	}
	ratio := w / h
	return ratio >= 0.7 && ratio <= 1.4
}

// replyContent draws a reply's text and the pictures it links to: a bare
// picture address shows its picture below the text once it loads. While it
// loads, or when it is not a picture after all, the address stays in the
// text as written.
func (t *TopicWindow) replyContent(c *ui.Context, content string, md *markdown.Options) {
	drawn := false
	start := 0
	for _, link := range pictureLinks(content) {
		img := avatarImage(t.proxy, link.url, t.invalidate)
		if img == nil {
			continue
		}

		if text := content[start:link.start]; strings.TrimSpace(text) != "" {
			markdown.View(c, text, md)
		}
		// A picture inside a sentence and about square is an emoji or a
		// sticker: it belongs beside the words, at their size. A transparent
		// one is the same even on a line of its own.
		emoticon := emoticonPicture(link.url) ||
			(inlinePictureLink(content, link) && squarePicture(img))
		if emoticon {
			ui.Image(c, img).Height(maxReplyEmoticonHeight).
				AlignSelf(ui.Start).
				Radius(4)
		} else {
			// A picture keeps its own size until it would be taller than the
			// cap; a taller one becomes a thumbnail that opens its original
			// on a click.
			_, height := ui.ImageSize(img)
			ui.Image(c, img).Height(min(height, maxReplyPictureHeight)).
				MaxWidthPercent(100).
				AlignSelf(ui.Start).
				Radius(8).
				Cursor(ui.CursorPointer).
				Tooltip("点击查看原图").
				OnClick(func() {
					go mygo.Shell.OpenExternal(link.url)
				})
		}
		start = link.end
		drawn = true
	}

	if !drawn {
		markdown.View(c, content, md)
		return
	}
	if tail := content[start:]; strings.TrimSpace(tail) != "" {
		markdown.View(c, tail, md)
	}
}

// replyTag draws a small pill in a reply's meta line, as the poster badge
// and the reply target use.
func (t *TopicWindow) replyTag(c *ui.Context, label string, fg, bg ui.Color) {
	ui.Box(c).Padding(1, 6).Radius(4).Background(bg).Children(func() {
		ui.Text(c, label).FontSize(10).TextColor(fg).SingleLine()
	})
}
