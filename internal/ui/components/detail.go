package components

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"mygo_demo/internal/api"
	"mygo_demo/internal/database/repo"
	"mygo_demo/internal/ui/components/icons"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/richtext"
)

// Detail draws the right column, whose content follows the sidebar's choice.
func (v *View) Detail(c *ui.Context) {
	t := c.Theme()

	switch v.Nav {
	case "profile", "settings":
		ui.Scroll(c).Grow(1).FillHeight().Children(func() {
			ui.Column(c).FillWidth().Padding(20).Gap(14).Background(t.Surface).Children(func() {
				if v.Nav == "profile" {
					v.profilePanel(c)
				} else {
					v.settingsPanel(c)
				}
			})
		})
	default:
		// The node detail fills the column instead of scrolling as a whole:
		// its header keeps its size and the topic list takes the rest. No
		// bottom padding, so the list runs to the window's edge.
		ui.Column(c).Grow(1).FillHeight().
			Padding(20, 20, 0, 20).
			Gap(14).
			Background(t.Surface).
			Children(func() {
				v.nodeDetail(c, v.selectedNode())
			})
	}
}

// nodeDetail draws the node and its newest topics as one panel: the
// overview on top, a hairline, then a list that scrolls to the panel's
// edge, so the two read together rather than as separate cards.
func (v *View) nodeDetail(c *ui.Context, n api.AllNodeInfo) {
	t := c.Theme()
	list := v.nodeTopicList(n.Name)
	topics, authors, loading, loadErr := list.snapshot()

	ui.Column(c).FillWidth().Grow(1).
		Padding(16).
		Radius(12, 12, 0, 0).
		Background(t.Background).
		Border(1, t.Border.Alpha(0.4)).
		Gap(12).
		Children(func() {
			v.nodeHeader(c, n)

			if header := plainText(n.Header); header != "" {
				ui.Text(c, header).FontSize(12).TextColor(t.TextMuted).MaxLines(2)
			}

			ui.Row(c).FillWidth().Wrap().Gap(8).Children(func() {
				v.statChip(c, icons.Topics, "主题总数", formatNumber(n.Topics))
				v.statChip(c, icons.Star, "收藏人数", formatNumber(n.Stars))
				v.statChip(c, icons.Nodes, "节点分类", nodeCategory(n))
			})

			ui.Box(c).FillWidth().Height(1).Background(t.Border.Alpha(0.25))

			ui.Row(c).FillWidth().AlignItems(ui.Center).Children(func() {
				ui.Text(c, "最新主题").FontSize(13).FontWeight(600).
					TextColor(t.Text).Grow(1)
				ui.Text(c, "来自 "+n.Name).FontSize(11).
					TextColor(t.TextMuted)
			})

			if len(topics) == 0 {
				v.topicListPlaceholder(c, loading, loadErr)
				return
			}

			ui.Scroll(c).FillWidth().Grow(1).TrackScroll(&v.TopicScroll).
				Children(func() {
					ui.Column(c).FillWidth().Padding(0, 12, 8, 0).
						Children(func() {
							for i, topic := range topics {
								v.topicRow(c, topic, authors[topic.ID])
								if i < len(topics)-1 {
									ui.Box(c).FillWidth().Height(1).
										Background(t.Border.Alpha(0.15))
								}
							}

							// Paging runs in the background; this keeps the
							// scroll's end telling the user more is coming.
							if loading {
								ui.Row(c).FillWidth().Padding(12).Center().Gap(8).
									Children(func() {
										ui.Spinner(c).Label("正在加载更多主题")
										ui.Text(c, "正在加载更多主题…").FontSize(11).
											TextColor(t.TextMuted)
									})
							}
							if loadErr != "" {
								ui.Text(c, "加载更多主题失败。").FontSize(11).
									TextColor(t.TextMuted)
							}
						})
				})

			// Loads the next page when the list is scrolled to its end.
			v.maybeLoadMoreTopics(n.Name)
		})
}

// topicListPlaceholder fills the list area while the first page is in
// flight: a turning spinner over skeleton rows, the pattern Linear and
// GitHub use to keep the list's shape while data arrives.
func (v *View) topicListPlaceholder(c *ui.Context, loading bool, loadErr string) {
	t := c.Theme()
	switch {
	case loading:
		ui.Column(c).FillWidth().Grow(1).Gap(14).Children(func() {
			ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(8).
				Children(func() {
					ui.Spinner(c).Label("正在加载主题")
					ui.Text(c, "正在加载最新主题…").FontSize(12).
						TextColor(t.TextMuted)
				})
			for range 3 {
				v.topicSkeleton(c)
			}
		})
	case loadErr != "":
		ui.Text(c, loadErr+"。").FontSize(12).TextColor(t.TextMuted)
	default:
		ui.Text(c, "暂时没有获取到主题。").FontSize(12).TextColor(t.TextMuted)
	}
}

// topicSkeleton is one placeholder row shaped like a topic row, so the
// list does not jump when the real rows arrive.
func (v *View) topicSkeleton(c *ui.Context) {
	t := c.Theme()
	ui.Row(c).FillWidth().Padding(8, 4).Gap(10).AlignItems(ui.Start).
		Children(func() {
			ui.Box(c).Size(26, 26).Radius(7).Background(t.Surface)
			ui.Column(c).Grow(1).Gap(7).Children(func() {
				ui.Box(c).Width(120).Height(10).Radius(4).Background(t.Surface)
				ui.Box(c).FillWidth().Height(12).Radius(4).Background(t.Surface)
				ui.Box(c).Width(180).Height(10).Radius(4).Background(t.Surface)
			})
		})
}

func (v *View) nodeHeader(c *ui.Context, n api.AllNodeInfo) {
	t := c.Theme()
	ui.Row(c).FillWidth().Gap(10).AlignItems(ui.Center).
		Children(func() {
			ui.Box(c).Size(36, 36).Radius(8).
				Background(t.Accent.Alpha(0.12)).Center().
				Children(func() {
					ui.Icon(c, icons.Nodes).FontSize(18).TextColor(t.Accent)
				})

			ui.Column(c).Grow(1).Gap(2).Children(func() {
				ui.Row(c).Gap(8).AlignItems(ui.Center).
					Children(func() {
						ui.Text(c, n.Title).FontSize(16).FontWeight(700).
							TextColor(t.Text).SingleLine()
						ui.Text(c, "/go/"+n.Name).FontSize(11).
							TextColor(t.TextMuted).SingleLine()
					})
			})

			v.favoriteButton(c, n)
		})
}

// favoriteButton toggles the node in the local favorites table.
func (v *View) favoriteButton(c *ui.Context, n api.AllNodeInfo) {
	t := c.Theme()

	starColor := t.Warning
	starBg := t.Warning.Alpha(0.12)
	starLabel := "已收藏"
	if !n.IsFavorite {
		starColor = t.TextMuted
		starBg = t.Surface
		starLabel = "收藏"
	}

	ui.ButtonBase(c).Label("收藏节点").
		Padding(5, 10).
		Radius(6).
		Background(starBg).
		Gap(4).
		AlignItems(ui.Center).
		Children(func() {
			if n.IsFavorite {
				ui.Icon(c, icons.StarFilled).FontSize(14).TextColor(starColor)
			} else {
				ui.Icon(c, icons.Star).FontSize(14).TextColor(starColor)
			}
			ui.Text(c, starLabel).FontSize(11).TextColor(starColor)
		}).
		OnClick(func() {
			if _, err := v.FavoritesRepo.ToggleFavorite(n.Name, n.Title); err != nil {
				v.ShowMessage("更新收藏失败")
				return
			}
			v.invalidate()
		})
}

// topicRow shows one topic the way a feed row does. The API 2.0 node topics
// carry no member, so the poster is read from the node's public page; a row
// without one falls back to a topic icon.
func (v *View) topicRow(c *ui.Context, topic api.TopicInfo, author *api.MemberInfo) {
	t := c.Theme()

	meta := timeAgo(topic.Created)
	if author != nil && author.Username != "" {
		meta = author.Username + " · " + meta
	}
	avatarURL := author.AvatarURL()

	ui.ButtonBase(c).FillWidth().
		Padding(8, 4).
		Gap(10).
		AlignItems(ui.Start).
		Children(func() {
			if avatarURL != "" {
				ui.Avatar(c, author.Username,
					v.nodeAvatarBitmap(v.ProxyURL, avatarURL)).Size(26, 26)
			} else {
				ui.Box(c).Size(26, 26).Radius(7).
					Background(t.Surface).Center().
					Children(func() {
						ui.Icon(c, icons.Topics).FontSize(13).TextColor(t.Accent)
					})
			}

			ui.Column(c).Grow(1).Gap(3).Children(func() {
				ui.Row(c).FillWidth().AlignItems(ui.Center).Children(func() {
					ui.Text(c, meta).FontSize(11).
						TextColor(t.TextMuted).SingleLine()
					ui.Spacer(c)
					ui.Box(c).Padding(1, 7).Radius(999).
						Background(t.Surface).
						Children(func() {
							ui.Text(c, strconv.Itoa(topic.Replies)).FontSize(10).
								TextColor(t.Accent)
						})
				})

				ui.Text(c, topic.Title).FontSize(13).FontWeight(600).
					TextColor(t.Text).MaxLines(2)

				if topic.LastReplyBy != "" {
					ui.Text(c, "最后回复 "+topic.LastReplyBy).FontSize(11).
						TextColor(t.TextMuted).SingleLine()
				}
			})
		}).
		OnClick(func() {
			OpenTopicWindow(v._client, v.ProxyURL, topic)
		})
}

// plainText strips the HTML the API returns for fields such as a node's
// header, keeping the readable text.
func plainText(s string) string {
	return strings.TrimSpace(richtext.ParseHTML(s).PlainText())
}

// topicURL returns the topic's site page, falling back to its id.
func topicURL(topic api.TopicInfo) string {
	if topic.URL != "" {
		return topic.URL
	}
	return "https://www.v2ex.com/t/" + strconv.Itoa(topic.ID)
}

// nodeCategory names the node's section, for the stats row. Root nodes
// report "v2ex" as their parent, which reads better spelled out.
func nodeCategory(n api.AllNodeInfo) string {
	switch n.ParentNodeName {
	case "":
		return "独立节点"
	case "v2ex":
		return "根节点"
	}
	return n.ParentNodeName
}

// statChip is one compact figure of the node card.
func (v *View) statChip(c *ui.Context, icon *ui.SVG, label, value string) {
	t := c.Theme()
	ui.Row(c).Gap(5).AlignItems(ui.Center).
		Padding(4, 9).
		Radius(999).
		Background(t.Surface).
		Children(func() {
			ui.Icon(c, icon).FontSize(11).TextColor(t.Accent)
			ui.Text(c, label).FontSize(11).TextColor(t.TextMuted)
			ui.Text(c, value).FontSize(11).FontWeight(600).
				TextColor(t.Text).SingleLine()
		})
}

func (v *View) profilePanel(c *ui.Context) {
	t := c.Theme()
	user := v.loadUser()

	ui.Column(c).FillWidth().
		Padding(24).
		Radius(12).
		Background(t.Background).
		Border(1, t.Border.Alpha(0.4)).
		Gap(16).
		Children(func() {
			if user == nil {
				ui.Text(c, "正在获取账号资料…").FontSize(13).TextColor(t.TextMuted)
				return
			}

			ui.Row(c).Gap(16).AlignItems(ui.Center).Children(func() {
				v.userAvatar(c)
				ui.Column(c).Gap(4).Children(func() {
					ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
						ui.Text(c, user.Username).FontSize(22).FontWeight(700).TextColor(t.Text)
						if user.Pro > 0 {
							ui.Badge(c, "PRO")
						}
					})
					ui.Text(c, "加入于 "+formatDate(user.Created)).FontSize(12).TextColor(t.TextMuted)
					ui.Text(c, "在 V2EX 打开主页 →").FontSize(12).TextColor(t.Accent).
						OnClick(func() {
							go mygo.Shell.OpenExternal(profileURL(user))
						})
				})
			})

			if user.Tagline != "" {
				ui.Text(c, user.Tagline).FontSize(12).TextColor(t.TextMuted)
			}

			v.profileInfo(c, user)

			ui.Box(c).FillWidth().Height(1).Background(t.Border.Alpha(0.25))
			ui.Text(c, "Biography").FontSize(11).FontWeight(600).TextColor(t.TextMuted)
			ui.Text(c, profileBio(user)).FontSize(13).TextColor(t.Text)
		})
}

// profileInfo lists the fields the member endpoint returns, skipping the
// ones the account left empty. Links open in the default browser.
func (v *View) profileInfo(c *ui.Context, user *api.ProfileResult) {
	ui.Column(c).FillWidth().Gap(6).Children(func() {
		v.profileRow(c, "用户 ID", strconv.Itoa(user.ID), "")
		if user.Location != "" {
			v.profileRow(c, "所在地", user.Location, "")
		}
		if user.Website != "" {
			v.profileRow(c, "个人网站", user.Website, user.Website)
		}
		if user.GitHub != nil && *user.GitHub != "" {
			v.profileRow(c, "GitHub", "@"+*user.GitHub, "https://github.com/"+*user.GitHub)
		}
		if user.Twitter != nil && *user.Twitter != "" {
			v.profileRow(c, "Twitter", "@"+*user.Twitter, "https://twitter.com/"+*user.Twitter)
		}
		if user.PSN != nil && *user.PSN != "" {
			v.profileRow(c, "PSN", *user.PSN, "https://psnprofiles.com/"+*user.PSN)
		}
		if user.BTC != nil && *user.BTC != "" {
			v.profileRow(c, "BTC", *user.BTC, "")
		}
		v.profileRow(c, "账号状态", profileStatus(user), "")
		v.profileRow(c, "资料更新", formatDate(user.LastModified), "")
	})
}

// profileRow draws one label and value; a non-empty link opens in the
// browser.
func (v *View) profileRow(c *ui.Context, label, value, link string) {
	t := c.Theme()
	ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(12).Children(func() {
		ui.Text(c, label).Width(72).FontSize(12).TextColor(t.TextMuted)
		text := ui.Text(c, value).FontSize(12).TextColor(t.Text).Grow(1).SingleLine()
		if link != "" {
			text.TextColor(t.Accent).OnClick(func() {
				go mygo.Shell.OpenExternal(link)
			})
		}
	})
}

// profileStatus names the account tier.
func profileStatus(user *api.ProfileResult) string {
	if user.Pro > 0 {
		return "PRO 会员"
	}
	return "普通会员"
}

// profileURL returns the site page of the account, falling back to the
// member path when the API did not include a URL.
func profileURL(user *api.ProfileResult) string {
	if user.URL != "" {
		return user.URL
	}
	return "https://www.v2ex.com/member/" + user.Username
}

// profileBio returns the account's bio, or a placeholder when it has none.
func profileBio(user *api.ProfileResult) string {
	if user.Bio != "" {
		return user.Bio
	}
	return "这个账号还没有填写个人简介。"
}

// formatDate renders a V2EX unix timestamp as a date, since the API returns
// seconds rather than a formatted string.
func formatDate(unix int64) string {
	if unix <= 0 {
		return "未知"
	}
	return time.Unix(unix, 0).Format("2006-01-02")
}

// formatDateTime renders a timestamp with its time of day.
func formatDateTime(unix int64) string {
	if unix <= 0 {
		return "未知"
	}
	return time.Unix(unix, 0).Format("2006-01-02 15:04")
}

// timeAgo renders a timestamp as how long ago it was, falling back to the
// date for anything older than a month.
func timeAgo(unix int64) string {
	if unix <= 0 {
		return "未知"
	}

	d := time.Since(time.Unix(unix, 0))
	switch {
	case d < time.Minute:
		return "刚刚"
	case d < time.Hour:
		return fmt.Sprintf("%d 分钟前", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%d 小时前", int(d.Hours()))
	case d < 30*24*time.Hour:
		return fmt.Sprintf("%d 天前", int(d.Hours()/24))
	}
	return formatDate(unix)
}

func (v *View) settingsPanel(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).FillWidth().
		Padding(24).
		Radius(12).
		Background(t.Background).
		Border(1, t.Border.Alpha(0.4)).
		Gap(14).
		Children(func() {
			ui.Text(c, "系统设置").FontSize(20).FontWeight(700).TextColor(t.Text)
			ui.Text(c, "Token、代理与本地数据设置。").FontSize(12).TextColor(t.TextMuted)
			ui.Box(c).FillWidth().Height(1).Background(t.Border.Alpha(0.25))

			v.settingRow(c, "Token 状态", tokenStatus(v._client.Key))
			v.proxySettingRow(c)
			v.settingRow(c, "节点缓存", v.nodeCacheSummary())
			v.settingRow(c, "外观", themeName())
		})
}

// proxySettingRow edits the proxy URL in place: Enter or 应用 validates it,
// switches the API client and avatar loader to it, and stores it for the
// next start.
func (v *View) proxySettingRow(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).FillWidth().
		Padding(10, 12).
		Radius(8).
		Background(t.Surface).
		Gap(6).
		Children(func() {
			ui.Text(c, "代理地址").FontSize(12).TextColor(t.TextMuted)

			ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
				field := ui.TextInput(c, &v.ProxyInput).
					FontSize(12).
					Height(32).
					Padding(4, 8).
					Placeholder("留空表示直连，例如 http://127.0.0.1:7890").
					Label("代理地址").
					Grow(1)
				apply := field.Submitted()

				ui.ButtonBase(c).Label("应用代理").
					Padding(4, 10).
					Radius(6).
					Background(t.Background).
					Border(1, t.Border.Alpha(0.4)).
					Children(func() {
						ui.Text(c, "应用").FontSize(11).TextColor(t.Text)
					}).
					OnClick(func() {
						v.applyProxy()
					})

				if apply {
					v.applyProxy()
				}
			})
		})
}

// applyProxy validates the draft, points the API client and avatar loader
// at it, and saves it. An empty value means a direct connection.
func (v *View) applyProxy() {
	proxy := strings.TrimSpace(v.ProxyInput)
	if proxy != "" && !strings.Contains(proxy, "://") {
		// Accept a bare host and port, as people write them, and show the
		// full URL once it is applied.
		proxy = "http://" + proxy
		v.ProxyInput = proxy
	}
	if proxy != "" {
		parsed, err := url.Parse(proxy)
		if err != nil || parsed.Scheme == "" || parsed.Host == "" {
			v.ShowMessage("代理地址格式不正确，例如 http://127.0.0.1:7890")
			return
		}
	}

	if err := v.SettingsRepo.Set(repo.ProxyKey, proxy); err != nil {
		v.ShowMessage("保存代理设置失败")
		return
	}

	v.ProxyURL = proxy
	if proxy == "" {
		v._client.RemoveProxy()
	} else {
		v._client.SetProxy(proxy)
	}
	v.ShowMessage("代理已更新")
	v.invalidate()
}

// tokenStatus names the token without printing it.
func tokenStatus(key string) string {
	if key == "" {
		return "未配置"
	}
	if len(key) < 12 {
		return "已配置"
	}
	return "已配置 · " + key[:4] + "…" + key[len(key)-4:]
}

// nodeCacheSummary says how many nodes the listing holds and how long the
// cache keeps it.
func (v *View) nodeCacheSummary() string {
	if n := v.nodeTotal(); n > 0 {
		return formatNumber(n) + " 个节点 · 缓存 " +
			strconv.Itoa(int(api.CacheTTL.Hours())) + " 小时"
	}
	return "尚未加载"
}

// themeName is the appearance the window shows now.
func themeName() string {
	if mygo.Theme.IsDark() {
		return "深色"
	}
	return "浅色"
}

func (v *View) settingRow(c *ui.Context, label, value string) {
	t := c.Theme()
	ui.Row(c).FillWidth().Padding(10, 12).Radius(8).Background(t.Surface).Children(func() {
		ui.Text(c, label).FontSize(12).TextColor(t.TextMuted).Grow(1)
		ui.Text(c, value).FontSize(12).TextColor(t.Text)
	})
}
