package components

import (
	"fmt"
	"sort"
	"strings"
	"sync"

	"mygo_demo/internal/api"
	"mygo_demo/internal/database"
	"mygo_demo/internal/database/repo"
	"mygo_demo/internal/ui/components/icons"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// View is the state the interface shows, built again every frame.
type View struct {
	Win *mygo.Window

	ProxyURL string
	// ProxyInput is the draft the settings panel edits; ApplyProxy copies it
	// to ProxyURL and stores it.
	ProxyInput string

	Query        string
	Nav          string // nodes | favorites | profile | settings
	SelectedNode string
	Filter       int // 0 all, 1 favorites

	_client *api.Client
	// ctx is the window's UI context, captured in Draw. It is stable for
	// the window's life and lets ShowToast queue a toast from any method
	// that runs while the view is building.
	ctx *ui.Context

	// MessageText is what the message drawer shows; empty, it is closed.
	MessageText string

	// nodeLayout is the node list's saved arrangement: pinned nodes and the
	// custom order of the rest. PinListState and NormalListState are its two
	// lists, the pinned section and everything else.
	nodeLayout      nodeLayout
	PinListState    ui.ListState
	NormalListState ui.ListState

	FavoritesRepo *repo.Favorites
	SettingsRepo  *repo.Settings

	Profile *api.Profile

	// User is the token owner's profile, fetched once; userLoaded marks that
	// the fetch ran, whether it succeeded or not.
	User       *api.ProfileResult
	userLoaded bool

	// nodeTopics caches each node's topic list so the detail panel does not
	// refetch it every frame, and keeps the paging state per node.
	nodeTopics  map[string]*nodeTopicList
	TopicScroll ui.ScrollState

	// allNodes caches the whole node listing, which backs the node list,
	// its search and the "全部" count; allNodesLoaded marks that the fetch
	// ran.
	allNodes       []api.AllNodeInfo
	allNodesLoaded bool
}

// nodeTopicList is one node's topic list and how much of it has loaded.
type nodeTopicList struct {
	// mu guards what a background fetch fills while the detail draws: the
	// topics, their posters, the paging state and the loading flags.
	mu      sync.Mutex
	topics  []api.TopicInfo
	authors map[int]*api.MemberInfo // topic id -> poster, from the node page
	page    int
	pages   int
	loading bool
	loadErr string

	// loaded marks that the first fetch started, so it is not started twice;
	// requested does the same for a load-more fetch. Both belong to the UI
	// thread.
	loaded    bool
	requested bool
}

// snapshot copies what the detail draws, so the background fetch can
// replace the slices and map without racing the frame.
func (l *nodeTopicList) snapshot() ([]api.TopicInfo, map[int]*api.MemberInfo, bool, string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	topics := append([]api.TopicInfo(nil), l.topics...)
	authors := make(map[int]*api.MemberInfo, len(l.authors))
	for id, member := range l.authors {
		authors[id] = member
	}
	return topics, authors, l.loading, l.loadErr
}

func NewView(client *api.Client, proxyURL string) *View {
	conn, err := database.Init()
	if err != nil {
		panic(err)
	}
	favoritesRepo := repo.NewFavorites(conn)
	settingsRepo := repo.NewSettings(conn)
	layout := loadNodeLayout(settingsRepo)

	return &View{
		_client:       client,
		Nav:           "nodes",
		SelectedNode:  "",
		FavoritesRepo: favoritesRepo,
		SettingsRepo:  settingsRepo,
		Profile:       api.NewProfile(client),
		ProxyURL:      proxyURL,
		ProxyInput:    proxyURL,
		nodeLayout:    layout,
	}
}

// Draw builds the whole window: a header across it, then the sidebar, the
// node list and the detail, side by side.
func (v *View) Draw(c *ui.Context) {
	v.ctx = c
	t := c.Theme()
	ui.Column(c).Fill().Background(t.Surface).Children(func() {
		v.NavComments(c)

		ui.Row(c).FillWidth().Grow(1).AlignItems(ui.Stretch).Children(func() {
			v.NavSidebar(c)
			v.NodeList(c)
			v.Detail(c)
		})
	})

	// Built last so the drawer shows above the content.
	v.Message(c)
}

// CurrentTheme returns the icon of the appearance the app is not in, as the
// theme button shows it.
func (v *View) CurrentTheme() *ui.SVG {
	return icons.Theme(mygo.Theme.IsDark())
}

func (v *View) invalidate() {
	if v.Win != nil {
		v.Win.Invalidate()
	}
}

func (v *View) navTitle() string {
	switch v.Nav {
	case "nodes":
		return "热门主题"
	case "favorites":
		return "收藏节点"
	case "profile":
		return "我的资料"
	case "settings":
		return "系统设置"
	}
	return "热门主题"
}

// selectedNode returns the node the detail panel shows. The selected node
// may come from a search or the favorites list, so it is looked up in the
// whole listing.
func (v *View) selectedNode() api.AllNodeInfo {
	var selected api.AllNodeInfo
	found := false

	if v.SelectedNode != "" {
		for _, n := range v.everyNode() {
			if n.Name == v.SelectedNode {
				selected, found = n, true
				break
			}
		}
	}
	if !found {
		// Nothing chosen yet: show the first row of the list, pinned first.
		rows := v.nodeRowsFor()
		if len(rows.pinned) > 0 {
			selected = rows.pinned[0]
		} else if len(rows.normal) > 0 {
			selected = rows.normal[0]
		}
	}

	if ok, err := v.FavoritesRepo.IsFavorite(selected.Name); err == nil {
		selected.IsFavorite = ok
	}
	return selected
}

// favoritesCount returns how many nodes the user has favorited. The table
// is local and small, so the query per frame is cheap.
func (v *View) favoritesCount() int {
	favorites, err := v.FavoritesRepo.Favorites()
	if err != nil {
		return 0
	}
	return len(favorites)
}

// everyNode returns the whole node listing, fetched once and kept in hot
// order. It backs the search, which must cover nodes no page has loaded
// yet, and the "全部" count. The listing is cached by the API client, so
// the fetch is one request at most.
func (v *View) everyNode() []api.AllNodeInfo {
	if !v.allNodesLoaded {
		v.allNodesLoaded = true
		nodes, err := api.NewAllNode(v._client).Get()
		if err != nil {
			return nil
		}
		sort.SliceStable(nodes, func(i, j int) bool {
			if nodes[i].Topics != nodes[j].Topics {
				return nodes[i].Topics > nodes[j].Topics
			}
			return nodes[i].Stars > nodes[j].Stars
		})
		v.allNodes = nodes
	}
	return v.allNodes
}

// nodeTotal returns how many nodes the whole listing holds.
func (v *View) nodeTotal() int {
	return len(v.everyNode())
}

// loadUser returns the token owner's profile, fetching it on the first
// call so no frame asks the API again.
func (v *View) loadUser() *api.ProfileResult {
	if v.userLoaded {
		return v.User
	}
	v.userLoaded = true

	if v.Profile == nil {
		v.Profile = api.NewProfile(v._client)
	}

	user, err := v.Profile.Get()
	if err != nil {
		v.ShowMessage("获取用户信息失败")
		return nil
	}
	v.User = user
	return user
}

// nodeTopicList returns the cached list for nodeName, fetching its first
// page and that page's posters on the first call. Later frames read the
// cache.
func (v *View) nodeTopicList(nodeName string) *nodeTopicList {
	if nodeName == "" {
		return &nodeTopicList{}
	}

	if v.nodeTopics == nil {
		v.nodeTopics = map[string]*nodeTopicList{}
	}
	list, ok := v.nodeTopics[nodeName]
	if !ok {
		list = &nodeTopicList{}
		v.nodeTopics[nodeName] = list
	}

	if !list.loaded {
		list.loaded = true
		list.mu.Lock()
		list.loading = true
		list.mu.Unlock()
		go v.fetchNodeTopics(nodeName, list, 1)
	}
	return list
}

// fetchNodeTopics loads one page into list in the background, replacing it
// on page 1 and appending after. The page's posters come along with it. The
// frame draws a loading state until the fetch redraws the window, so a slow
// request never stalls the interface.
func (v *View) fetchNodeTopics(nodeName string, list *nodeTopicList, page int) {
	topics, pagination, err := api.NewNode(v._client).Topics(nodeName, page)
	if err != nil {
		list.mu.Lock()
		list.loading = false
		list.loadErr = "获取节点主题失败"
		list.mu.Unlock()
		v.invalidate()
		return
	}

	// A failed poster read is not fatal: those rows fall back to the topic
	// icon.
	posters, _ := api.NewTopic(v._client).NodePagePosters(nodeName, page)

	list.mu.Lock()
	list.loadErr = ""
	if page == 1 {
		list.topics = topics
	} else {
		list.topics = append(list.topics, topics...)
	}
	list.page = page
	list.pages = pagination.Pages
	if len(posters) > 0 {
		if list.authors == nil {
			list.authors = make(map[int]*api.MemberInfo, len(posters))
		}
		for id, member := range posters {
			list.authors[id] = member
		}
	}
	list.loading = false
	list.mu.Unlock()

	v.invalidate()
}

// maybeLoadMoreTopics appends the next page when the detail's topic list
// reaches its end. The guard clears once the list is no longer at the end,
// so one arrival loads one page; the page itself is fetched in the
// background.
func (v *View) maybeLoadMoreTopics(nodeName string) {
	list := v.nodeTopics[nodeName]
	if list == nil || !list.loaded {
		return
	}

	if v.TopicScroll.MaxY <= 0 || v.TopicScroll.Y < v.TopicScroll.MaxY {
		list.requested = false
		return
	}

	list.mu.Lock()
	if list.requested || list.loading || list.page >= list.pages {
		list.mu.Unlock()
		return
	}
	page := list.page + 1
	list.loading = true
	list.mu.Unlock()

	list.requested = true
	go v.fetchNodeTopics(nodeName, list, page)
}

// fadeDivider paints a border that is transparent at both ends and takes
// the theme's border color in the middle, so vertical separators fade out
// instead of running edge to edge.
func fadeDivider(t *ui.Theme) ui.LinearGradient {
	return ui.LinearGradient{
		Angle: 180,
		Stops: []ui.GradientStop{
			{Color: t.Border.Alpha(0), At: 0},
			{Color: t.Border.Alpha(0.25), At: 0.5},
			{Color: t.Border.Alpha(0), At: 1},
		},
	}
}

func formatNumber(n int) string {
	s := fmt.Sprintf("%d", n)
	if len(s) <= 3 {
		return s
	}
	var b strings.Builder
	for i, r := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(r)
	}
	return b.String()
}
