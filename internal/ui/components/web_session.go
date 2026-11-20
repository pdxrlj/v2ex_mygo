package components

import (
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"mygo_demo/internal/api"
	"mygo_demo/internal/database/repo"
	"mygo_demo/internal/secure"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// loadWebSettings reads the saved web session cookie, decrypting it, and the
// daily check-in switch.
func loadWebSettings(settings *repo.Settings) (cookie string, autoSignIn bool) {
	if value, ok, err := settings.Get(repo.WebCookieKey); err == nil && ok && value != "" {
		if encrypted, err := base64.StdEncoding.DecodeString(value); err == nil {
			if plain, err := secure.Unprotect(encrypted); err == nil {
				cookie = string(plain)
			}
		}
	}

	if value, ok, err := settings.Get(repo.AutoSignInKey); err == nil && ok {
		autoSignIn = value == "1"
	}
	return cookie, autoSignIn
}

// webSnapshot copies what the settings card draws, so the background
// requests can replace it without racing the frame.
func (v *View) webSnapshot() (cookie, balance string, busy bool, notice string, noticeOK bool) {
	v.webMu.Lock()
	defer v.webMu.Unlock()
	return v.webCookie, v.webBalance, v.webBusy, v.webNotice, v.webNoticeOK
}

// saveWebCookie keeps a refreshed session cookie, encrypted at rest, and
// bumps the epoch so a request of the old session cannot write its result.
func (v *View) saveWebCookie(cookie string) {
	if cookie == "" {
		return
	}

	v.webMu.Lock()
	v.webCookie = cookie
	v.webEpoch++
	v.webMu.Unlock()

	encrypted, err := secure.Protect([]byte(cookie))
	if err != nil {
		return
	}
	_ = v.SettingsRepo.Set(repo.WebCookieKey,
		base64.StdEncoding.EncodeToString(encrypted))
}

// forgetWebCookie drops the session, in memory and at rest.
func (v *View) forgetWebCookie() {
	v.webMu.Lock()
	v.webCookie = ""
	v.webEpoch++
	v.webMu.Unlock()
	_ = v.SettingsRepo.Set(repo.WebCookieKey, "")
}

// webEpochCurrent tells whether the session that started a request is still
// the one the card shows.
func (v *View) webEpochCurrent(epoch int) bool {
	v.webMu.Lock()
	defer v.webMu.Unlock()
	return v.webEpoch == epoch
}

// finishWeb stores what a finished request shows: the busy flag clears, the
// balance replaces the old one when the request carried it, and the notice
// reports the outcome.
func (v *View) finishWeb(balance, notice string, ok bool) {
	v.webMu.Lock()
	v.webBusy = false
	if balance != "" {
		v.webBalance = balance
	}
	v.webNotice = notice
	v.webNoticeOK = ok
	v.webMu.Unlock()
}

// setWebNotice reports an outcome without changing the balance.
func (v *View) setWebNotice(notice string, ok bool) {
	v.webMu.Lock()
	v.webNotice = notice
	v.webNoticeOK = ok
	v.webMu.Unlock()
}

// webRequestFailed reports a failed page request, and forgets a session the
// site no longer accepts.
func (v *View) webRequestFailed(err error, action string) {
	if errors.Is(err, api.ErrWebSignedOut) {
		v.forgetWebCookie()
		v.finishWeb("", "登录已失效，请重新登录", false)
	} else {
		v.finishWeb("", action+"："+err.Error(), false)
	}
	v.invalidate()
}

// autoSignInCheckInterval is how often the background loop asks whether
// today's bonus is still unclaimed; the settings read is cheap, and a day
// with failures retries at this pace.
const autoSignInCheckInterval = 30 * time.Minute

// autoSignInLoop claims the daily bonus when the switch is on, the session
// exists, and today has not been claimed; it checks now and then every half
// hour, so an app left open across midnight still claims the new day.
func (v *View) autoSignInLoop() {
	ticker := time.NewTicker(autoSignInCheckInterval)
	defer ticker.Stop()

	for {
		v.maybeAutoSignIn()
		<-ticker.C
	}
}

// maybeAutoSignIn starts today's check-in in the background when it is due.
func (v *View) maybeAutoSignIn() {
	v.webMu.Lock()
	enabled := v.autoSignInOn
	cookie := v.webCookie
	proxy := v.webProxy
	v.webMu.Unlock()

	if !enabled || cookie == "" {
		return
	}
	if value, ok, err := v.SettingsRepo.Get(repo.LastSignInKey); err == nil &&
		ok && value == time.Now().Format("2006-01-02") {
		return
	}
	go v.signInNow(proxy)
}

// maybeRefreshWebBalanceForProfile reads the balance once when the member is
// signed in through the web login and it is still unknown, so the profile
// page can show it.
func (v *View) maybeRefreshWebBalanceForProfile() {
	v.webMu.Lock()
	fetch := v.webCookie != "" && v.webBalance == "" && !v.webBusy && !v.webBalanceRequested
	if fetch {
		v.webBalanceRequested = true
	}
	proxy := v.webProxy
	v.webMu.Unlock()

	if fetch {
		go v.refreshWebBalance(proxy)
	}
}

// signInNow claims the daily bonus with the saved cookie and reads the
// balance after it, in the background.
func (v *View) signInNow(proxy string) {
	v.webMu.Lock()
	if v.webBusy {
		v.webMu.Unlock()
		return
	}
	cookie := v.webCookie
	v.webBusy = true
	v.webBalanceRequested = true
	epoch := v.webEpoch
	v.webMu.Unlock()

	if cookie == "" {
		v.finishWeb("", "请先完成网页登录", false)
		v.invalidate()
		return
	}

	session, err := api.NewWebSession(proxy, cookie)
	if err != nil {
		v.finishWeb("", "创建网页会话失败", false)
		v.invalidate()
		return
	}

	already, days, err := session.SignIn()
	if err != nil {
		if v.webEpochCurrent(epoch) {
			v.webRequestFailed(err, "签到失败")
		}
		return
	}

	balance, _ := session.Balance()
	if !v.webEpochCurrent(epoch) {
		return // the session was replaced or dropped while signing in
	}
	v.saveWebCookie(session.CookieHeader())
	_ = v.SettingsRepo.Set(repo.LastSignInKey, time.Now().Format("2006-01-02"))

	notice := "签到成功"
	if already {
		notice = "今天已经签到过了"
	}
	if days > 0 {
		notice += fmt.Sprintf("，已连续 %d 天", days)
	}
	v.finishWeb(balance, notice, true)
	v.invalidate()
}

// refreshWebBalance reads the coin balance with the saved cookie, in the
// background.
func (v *View) refreshWebBalance(proxy string) {
	v.webMu.Lock()
	if v.webBusy {
		v.webMu.Unlock()
		return
	}
	cookie := v.webCookie
	v.webBusy = true
	v.webBalanceRequested = true
	epoch := v.webEpoch
	v.webMu.Unlock()

	if cookie == "" {
		v.finishWeb("", "请先完成网页登录", false)
		v.invalidate()
		return
	}

	session, err := api.NewWebSession(proxy, cookie)
	if err != nil {
		v.finishWeb("", "创建网页会话失败", false)
		v.invalidate()
		return
	}

	balance, err := session.Balance()
	if err != nil {
		if v.webEpochCurrent(epoch) {
			v.webRequestFailed(err, "读取余额失败")
		}
		return
	}
	if !v.webEpochCurrent(epoch) {
		return
	}
	v.saveWebCookie(session.CookieHeader())
	v.finishWeb(balance, "余额已更新", true)
	v.invalidate()
}

// onWebLogin takes the cookie the login window captured and reads the
// balance with it, which also proves the session works.
func (v *View) onWebLogin(cookie, proxy string) {
	if cookie == "" {
		v.setWebNotice("未能打开登录页或取得 Cookie，请检查网络、代理与 WebView2 运行时后重试", false)
		v.invalidate()
		return
	}

	v.saveWebCookie(cookie)
	v.webMu.Lock()
	v.webBalance = ""
	v.webBusy = false
	v.webMu.Unlock()
	v.setWebNotice("登录成功，正在读取余额…", true)
	go v.refreshWebBalance(proxy)
	v.invalidate()
	invalidateTopicWindow()
}

// clearWebSession signs the saved session out and deletes the site's cookies
// from the app's web views, so the next login starts fresh.
func (v *View) clearWebSession() {
	v.forgetWebCookie()
	v.webMu.Lock()
	v.webBalance = ""
	v.webBusy = false
	v.webBalanceRequested = false
	v.webMu.Unlock()
	v.setWebNotice("已退出网页登录", true)

	go func() {
		cookies, err := mygo.Cookies.List(mygo.CookieFilter{Domain: "v2ex.com"})
		if err != nil {
			return
		}
		for _, c := range cookies {
			_ = mygo.Cookies.Delete(c)
		}
	}()

	v.invalidate()
	invalidateTopicWindow()
}

// applyAutoSignIn saves the switch and runs today's check-in when it is
// turned on.
func (v *View) applyAutoSignIn() {
	value := "0"
	if v.AutoSignIn {
		value = "1"
	}
	_ = v.SettingsRepo.Set(repo.AutoSignInKey, value)

	v.webMu.Lock()
	v.autoSignInOn = v.AutoSignIn
	enabled := v.AutoSignIn
	cookie := v.webCookie
	v.webMu.Unlock()

	if enabled && cookie != "" {
		go v.signInNow(v.ProxyURL)
	}

	v.ShowToast("每日自动签到设置已更新")
	v.invalidate()
}

// webSessionCard edits the web session: the login, the daily check-in, the
// balance and the automatic check-in switch.
func (v *View) webSessionCard(c *ui.Context) {
	t := c.Theme()
	cookie, balance, busy, notice, noticeOK := v.webSnapshot()

	ui.Column(c).FillWidth().
		Padding(12, 14).
		Radius(8).
		Background(t.Surface).
		Gap(10).
		Children(func() {
			ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(10).Children(func() {
				ui.Column(c).Grow(1).Gap(3).Children(func() {
					ui.Text(c, "V2EX 网页登录").FontSize(13).
						FontWeight(600).TextColor(t.Text)
					ui.Text(c, "用于每日签到和铜币余额；Cookie 以 Windows DPAPI 加密保存在本机。").
						FontSize(11).TextColor(t.TextMuted)
				})

				status := "未登录"
				color := t.TextMuted
				if cookie != "" {
					status = "已登录"
					color = t.Accent
				}
				ui.Box(c).Padding(1, 8).Radius(999).
					Background(color.Alpha(0.12)).
					Children(func() {
						ui.Text(c, status).FontSize(10).TextColor(color)
					})
			})

			ui.Row(c).FillWidth().Gap(6).AlignItems(ui.Center).Children(func() {
				ui.Text(c, "金币余额").FontSize(11).TextColor(t.TextMuted)
				value := balance
				if value == "" {
					value = "—"
				}
				ui.Text(c, value).FontSize(12).FontWeight(600).TextColor(t.Text)
			})

			ui.Row(c).FillWidth().Wrap().Gap(8).Children(func() {
				label := "网页登录"
				if cookie != "" {
					label = "重新登录"
				}
				v.webActionButton(c, label, func() {
					proxy := v.ProxyURL
					OpenWebLoginWindow(func(cookie string) {
						v.onWebLogin(cookie, proxy)
					})
				})
				if cookie != "" {
					v.webActionButton(c, "签到", func() {
						go v.signInNow(v.ProxyURL)
					})
					v.webActionButton(c, "刷新余额", func() {
						go v.refreshWebBalance(v.ProxyURL)
					})
					v.webActionButton(c, "退出登录", v.clearWebSession)
				}
			})

			ui.Box(c).FillWidth().Height(1).Background(t.Border.Alpha(0.2))

			ui.Row(c).FillWidth().AlignItems(ui.Center).Children(func() {
				ui.Column(c).Grow(1).Gap(2).Children(func() {
					ui.Text(c, "每日自动签到").FontSize(12).TextColor(t.Text)
					ui.Text(c, "启动时检查当天是否已领取").
						FontSize(11).TextColor(t.TextMuted)
				})

				toggle := ui.Switch(c, &v.AutoSignIn).Label("每日自动签到")
				if toggle.Changed() {
					v.applyAutoSignIn()
				}
			})

			if busy {
				ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(8).Children(func() {
					ui.Spinner(c).Label("正在请求 V2EX")
					ui.Text(c, "正在请求 V2EX…").FontSize(11).TextColor(t.TextMuted)
				})
			}
			if notice != "" {
				color := t.Accent
				if !noticeOK {
					color = t.Warning
				}
				ui.Text(c, notice).FontSize(11).TextColor(color)
			}
		})
}

// webActionButton is one small action of the web session card.
func (v *View) webActionButton(c *ui.Context, label string, action func()) {
	t := c.Theme()
	ui.ButtonBase(c).Label(label).
		Padding(5, 10).
		Radius(6).
		Background(t.Background).
		Border(1, t.Border.Alpha(0.4)).
		Children(func() {
			ui.Text(c, label).FontSize(11).TextColor(t.Text)
		}).
		OnClick(action)
}
