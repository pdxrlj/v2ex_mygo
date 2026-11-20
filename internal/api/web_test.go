package api

import "testing"

// TestParseBalanceHTMLModern checks the coin block the balance page shows,
// whose unit lives in an icon's alt text.
func TestParseBalanceHTMLModern(t *testing.T) {
	html := `<div class="balance_area bigger">` +
		`<span class="fade">1234 <img src="/static/img/coin_b.png" alt="B"></span>` +
		`<span class="fade">56 <img src="/static/img/coin_s.png" alt="S"></span>` +
		`</div>`
	if got := parseBalanceHTML(html); got != "1234 铜币 · 56 银币" {
		t.Errorf("parseBalanceHTML() = %q", got)
	}
}

// TestParseBalanceHTMLTable checks the older table layout's total.
func TestParseBalanceHTMLTable(t *testing.T) {
	html := `<table><tr><td class="d" style="text-align: right;">12.5</td></tr></table>`
	if got := parseBalanceHTML(html); got != "12.5 铜币" {
		t.Errorf("parseBalanceHTML() = %q", got)
	}
}

// TestParseBalanceHTMLNone checks that a page without a balance reads as "".
func TestParseBalanceHTMLNone(t *testing.T) {
	if got := parseBalanceHTML(`<html><body>nothing</body></html>`); got != "" {
		t.Errorf("parseBalanceHTML() = %q, want empty", got)
	}
}

// TestParseReplyOnce checks the reply form's one-time code, whose attributes
// the site writes in varying orders.
func TestParseReplyOnce(t *testing.T) {
	forms := []string{
		`<input type="hidden" value="88992" id="once" name="once" />`,
		`<input type="hidden" name="once" value="88992" />`,
		`<input name="once" value="88992" type="hidden">`,
	}
	for _, html := range forms {
		if got := parseReplyOnce(html); got != "88992" {
			t.Errorf("parseReplyOnce(%q) = %q, want 88992", html, got)
		}
	}
	if got := parseReplyOnce(`<input name="other" value="1">`); got != "" {
		t.Errorf("parseReplyOnce(no once) = %q, want empty", got)
	}
}

// TestParseCookieHeader checks that a saved Cookie header becomes cookies,
// values with an "=" included, and that RFC 6265 quotes are stripped.
func TestParseCookieHeader(t *testing.T) {
	cookies := parseCookieHeader(`A2="abc"; PB3_SESSION=def=ghi; bad; =empty; V2EX_TAB="z"`)
	if len(cookies) != 3 {
		t.Fatalf("got %d cookies, want 3", len(cookies))
	}
	if cookies[0].Name != "A2" || cookies[0].Value != "abc" {
		t.Errorf("first cookie = %+v", cookies[0])
	}
	if cookies[1].Name != "PB3_SESSION" || cookies[1].Value != "def=ghi" {
		t.Errorf("second cookie = %+v", cookies[1])
	}
	if cookies[2].Name != "V2EX_TAB" || cookies[2].Value != "z" {
		t.Errorf("third cookie = %+v", cookies[2])
	}
}

// TestParseDailyDays checks the streak of the mission page.
func TestParseDailyDays(t *testing.T) {
	if got := parseDailyDays(`<div>已连续登录 123 天</div>`); got != 123 {
		t.Errorf("parseDailyDays() = %d, want 123", got)
	}
	if got := parseDailyDays(`<div>连续签到：7 天</div>`); got != 7 {
		t.Errorf("parseDailyDays() = %d, want 7", got)
	}
	if got := parseDailyDays(`<div>nothing</div>`); got != 0 {
		t.Errorf("parseDailyDays() = %d, want 0", got)
	}
}
