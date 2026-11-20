package components

import (
	"log"
	"net/url"
	"os"
	"strings"
	"sync"

	"mygo_demo/assets/img"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// webSignInURL is the site's sign-in page the login window opens.
const webSignInURL = "https://www.v2ex.com/signin"

// webLoginWindow is the login window the app keeps open, at most one.
var webLoginWindow *mygo.Window

// webLoginStarted marks that a login web view was made, which starts the
// shared WebView2 environment. Its browser reads the proxy arguments once,
// so a proxy changed after this needs a restart to reach the login page.
var webLoginStarted bool

// ConfigureWebViewProxy points WebView2's pages at the proxy the API uses.
// WebView2 reads the variable when it makes its environment, so call this
// before the first web view of the run to have it apply.
func ConfigureWebViewProxy(proxy string) {
	args := "--no-proxy-server"
	if proxy != "" {
		args = "--proxy-server=" + proxy
	}
	if existing := strings.TrimSpace(os.Getenv("WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS")); existing != "" {
		args = existing + " " + args
	}
	_ = os.Setenv("WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS", args)
}

// OpenWebLoginWindow shows V2EX's sign-in page in a window of its own and
// hands the session cookie to onDone once the member is signed in. The
// member types the password and the captcha in the page, so the app never
// sees either.
func OpenWebLoginWindow(onDone func(cookie string)) {
	if webLoginWindow != nil {
		webLoginWindow.Close()
		webLoginWindow = nil
	}

	var (
		view     *mygo.WebView
		captured bool
		statusMu sync.Mutex
		status   string
	)

	capture := func() {
		if captured {
			return
		}
		cookie := readV2EXCookie()
		if cookie == "" {
			return
		}
		captured = true
		onDone(cookie)
		if webLoginWindow != nil {
			webLoginWindow.Close()
		}
	}

	win := mygo.NewWindow(mygo.WindowOptions{
		Title:           "登录 V2EX",
		Width:           520,
		Height:          680,
		MinWidth:        420,
		MinHeight:       520,
		BackgroundColor: "light-dark(#f5f6f8, #16181c)",
		Content: ui.View(func(c *ui.Context) {
			t := c.Theme()
			ui.Column(c).Fill().Children(func() {
				ui.Row(c).FillWidth().Padding(12, 14).Gap(10).
					AlignItems(ui.Center).
					Background(t.Surface).
					Children(func() {
						ui.Text(c, "登录 V2EX").FontSize(14).
							FontWeight(700).TextColor(t.Text)
						ui.Spacer(c)

						statusMu.Lock()
						hint := status
						statusMu.Unlock()
						hintColor := t.TextMuted
						if hint == "" {
							hint = "登录成功后窗口会自动关闭"
						} else {
							hintColor = t.Warning
						}
						ui.Text(c, hint).FontSize(11).TextColor(hintColor)

						ui.ButtonBase(c).Label("已完成登录").
							Padding(4, 10).
							Radius(6).
							Background(t.Background).
							Border(1, t.Border.Alpha(0.4)).
							Children(func() {
								ui.Text(c, "已完成登录").FontSize(11).
									TextColor(t.Text)
							}).
							OnClick(capture)
					})

				if view != nil {
					ui.WebView(c, view).FillWidth().Grow(1)
				}
			})
		}),
	})

	wv, err := win.NewWebView(mygo.WebViewOptions{URL: webSignInURL})
	if err != nil {
		log.Printf("login web view: %v", err)
		win.Close()
		onDone("")
		return
	}
	view = wv
	webLoginStarted = true

	// The page lands on a signed-in page once the login took; the cookie is
	// written by then, so it can be read and the window closed. A navigation
	// that came too early finds no cookie and the next one tries again.
	wv.Page().OnDidNavigate(func(rawURL string) {
		statusMu.Lock()
		status = ""
		statusMu.Unlock()
		win.Invalidate()

		if !isSignedInPage(rawURL) {
			return
		}
		go capture()
	})
	wv.Page().OnDidFailLoad(func(*mygo.LoadError) {
		statusMu.Lock()
		status = "页面加载失败，请检查网络或代理设置"
		statusMu.Unlock()
		win.Invalidate()
	})

	webLoginWindow = win
	win.OnClosed(func() {
		if webLoginWindow == win {
			webLoginWindow = nil
		}
	})
	if err := win.SetIcon(img.LogoPNG); err != nil {
		log.Printf("login window icon: %v", err)
	}
}

// isSignedInPage tells whether url is a page of the site other than the
// sign-in page, which is where the login lands when it worked.
func isSignedInPage(rawURL string) bool {
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	if u.Host != "www.v2ex.com" && u.Host != "v2ex.com" {
		return false
	}
	return !strings.Contains(u.Path, "/signin")
}

// readV2EXCookie returns the site's session cookies as a Cookie header, once
// the login cookie A2 is among them.
func readV2EXCookie() string {
	cookies, err := mygo.Cookies.List(mygo.CookieFilter{
		URL: "https://www.v2ex.com/",
	})
	if err != nil {
		return ""
	}

	signedIn := false
	parts := make([]string, 0, len(cookies))
	for _, c := range cookies {
		value := strings.TrimSpace(c.Value)
		// WebView2 can keep a cookie's RFC 6265 quotes, while a Cookie
		// header sends the value bare; keeping them makes net/http drop
		// them and log every request.
		if len(value) >= 2 && value[0] == '"' && value[len(value)-1] == '"' {
			value = value[1 : len(value)-1]
		}
		if c.Name == "A2" && value != "" {
			signedIn = true
		}
		parts = append(parts, c.Name+"="+value)
	}
	if !signedIn {
		return ""
	}
	return strings.Join(parts, "; ")
}
