package api

import (
	"errors"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/go-resty/resty/v2"
)

// ErrWebSignedOut reports that a web request ended at the sign-in page: the
// saved cookie no longer signs the member in.
var ErrWebSignedOut = errors.New("v2ex web session expired")

// webUserAgent is a browser's user agent, which the site's pages expect;
// Go's default one gets a bare page.
const webUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) " +
	"AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36"

var (
	// dailyOncePattern finds the one-time code of the daily bonus button.
	dailyOncePattern = regexp.MustCompile(`once=(\d+)`)
	// dailyDaysPattern finds the streak the mission page shows.
	dailyDaysPattern = regexp.MustCompile(`(?:已连续登录|连续签到)[^0-9]{0,12}(\d+)\s*天`)
	// balanceBlockPattern finds the coin block at the top of the balance
	// page, whose coins carry their unit in an icon's alt text.
	balanceBlockPattern = regexp.MustCompile(`(?s)balance_area bigger.*?</div>`)
	// balanceCoinPattern finds one coin amount of that block.
	balanceCoinPattern = regexp.MustCompile(`(\d+(?:\.\d+)?)\s*<img[^>]+alt="([BGS])"`)
	// balanceCellPattern finds the total of the older table layout.
	balanceCellPattern = regexp.MustCompile(`<td class="d" style="text-align: right;">(\d+(?:\.\d+)?)</td>`)
)

// WebSession is a session on the V2EX web pages, signed in by the cookies a
// browser login leaves. It backs what API 2.0 does not offer: the daily
// bonus and the coin balance. The client keeps the cookie jar, so a cookie
// the site renews on a response replaces the old one.
type WebSession struct {
	client *resty.Client
	jar    *cookiejar.Jar
}

// NewWebSession returns a web session signed in with cookieHeader, as the
// login window captured it, routed through proxy when proxy is set.
func NewWebSession(proxy string, cookieHeader string) (*WebSession, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}
	site, err := url.Parse(siteURL + "/")
	if err != nil {
		return nil, err
	}
	jar.SetCookies(site, parseCookieHeader(cookieHeader))

	client := resty.New().
		SetBaseURL(siteURL).
		SetCookieJar(jar).
		SetHeader("User-Agent", webUserAgent).
		SetHeader("Accept-Language", "zh-CN,zh;q=0.9")
	if proxy != "" {
		client.SetProxy(proxy)
	}

	return &WebSession{client: client, jar: jar}, nil
}

// CookieHeader returns the session's current cookies, so the ones the site
// renewed can be saved again.
func (w *WebSession) CookieHeader() string {
	site, err := url.Parse(siteURL + "/")
	if err != nil {
		return ""
	}

	cookies := w.jar.Cookies(site)
	parts := make([]string, 0, len(cookies))
	for _, c := range cookies {
		parts = append(parts, c.Name+"="+c.Value)
	}
	return strings.Join(parts, "; ")
}

// SignIn claims the daily bonus. already reports that it was claimed before
// today, and days is the streak the site shows, 0 when it names none.
func (w *WebSession) SignIn() (already bool, days int, err error) {
	daily, err := w.get("/mission/daily", "")
	if err != nil {
		return false, 0, err
	}
	if w.signedOut(daily) {
		return false, 0, ErrWebSignedOut
	}

	html := daily.String()
	days = parseDailyDays(html)
	if strings.Contains(html, "每日登录奖励已领取") {
		return true, days, nil
	}

	m := dailyOncePattern.FindStringSubmatch(html)
	if len(m) != 2 {
		return false, days, errors.New("未找到签到入口")
	}

	redeem := siteURL + "/mission/daily/redeem?once=" + m[1]
	if _, err := w.get(redeem, siteURL+"/mission/daily"); err != nil {
		return false, days, err
	}

	// The mission page flips its button once the bonus is claimed, which
	// tells whether the redeem call went through.
	confirm, err := w.get("/mission/daily", "")
	if err == nil && strings.Contains(confirm.String(), "每日登录奖励已领取") {
		days = parseDailyDays(confirm.String())
	}
	return false, days, nil
}

// Balance returns the coin balance as the site shows it, for example
// "1234 铜币 · 56 银币", or an error when the page cannot be read.
func (w *WebSession) Balance() (string, error) {
	resp, err := w.get("/balance", "")
	if err != nil {
		return "", err
	}
	if w.signedOut(resp) {
		return "", ErrWebSignedOut
	}

	balance := parseBalanceHTML(resp.String())
	if balance == "" {
		return "", errors.New("无法解析余额页面")
	}
	return balance, nil
}

// get reads a page of the site, with an optional referer the site checks.
func (w *WebSession) get(path, referer string) (*resty.Response, error) {
	req := w.client.R()
	if referer != "" {
		req.SetHeader("Referer", referer)
	}
	return req.Get(path)
}

// signedOut tells whether the response ended at the sign-in page.
func (w *WebSession) signedOut(resp *resty.Response) bool {
	if resp == nil {
		return false
	}
	if raw := resp.RawResponse; raw != nil && raw.Request != nil && raw.Request.URL != nil {
		if strings.Contains(raw.Request.URL.Path, "/signin") {
			return true
		}
	}
	return strings.Contains(resp.String(), "你要查看的页面需要先登录")
}

// parseCookieHeader turns "name=value; other=value" into cookies.
func parseCookieHeader(header string) []*http.Cookie {
	var cookies []*http.Cookie
	for _, part := range strings.Split(header, ";") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		name, value, ok := strings.Cut(part, "=")
		if !ok || name == "" {
			continue
		}
		value = strings.TrimSpace(value)
		// A saved value may still carry the quotes an RFC 6265 cookie may
		// be wrapped in; the header sends it bare.
		if len(value) >= 2 && value[0] == '"' && value[len(value)-1] == '"' {
			value = value[1 : len(value)-1]
		}
		cookies = append(cookies, &http.Cookie{Name: name, Value: value})
	}
	return cookies
}

// parseDailyDays returns the streak of the mission page, or 0.
func parseDailyDays(html string) int {
	m := dailyDaysPattern.FindStringSubmatch(html)
	if len(m) != 2 {
		return 0
	}
	days, err := strconv.Atoi(m[1])
	if err != nil {
		return 0
	}
	return days
}

// parseBalanceHTML returns the balance of the balance page, or "" when the
// page names none.
func parseBalanceHTML(html string) string {
	if block := balanceBlockPattern.FindString(html); block != "" {
		units := map[string]string{"B": "铜币", "S": "银币", "G": "金币"}
		var parts []string
		for _, m := range balanceCoinPattern.FindAllStringSubmatch(block, -1) {
			if unit, ok := units[m[2]]; ok {
				parts = append(parts, m[1]+" "+unit)
			}
		}
		if len(parts) > 0 {
			return strings.Join(parts, " · ")
		}
	}

	// The older page listed the totals as table cells; the last one is the
	// balance.
	if cells := balanceCellPattern.FindAllStringSubmatch(html, -1); len(cells) > 0 {
		return cells[len(cells)-1][1] + " 铜币"
	}
	return ""
}
