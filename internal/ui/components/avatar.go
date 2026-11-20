package components

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/egoist/mygo/ui"
)

// maxAvatarCache caps the decoded pictures kept in memory: a long session
// scrolling nodes and replies must not grow the cache without end. The
// oldest go first and are downloaded again if a later frame shows them.
const maxAvatarCache = 240

// avatars caches node avatars by URL. Decoding happens off the UI thread, so
// a row draws its fallback icon until the picture is ready; a failed load is
// cached too, which keeps a broken URL from being retried every frame.
var avatars = struct {
	sync.Mutex
	images map[string]ui.ImageSource
	order  []string
	busy   map[string]bool
}{images: map[string]ui.ImageSource{}, busy: map[string]bool{}}

// avatarClients caches one HTTP client per proxy. Downloads do not reuse the
// API client: it carries the V2EX bearer token and the CDN is another host.
var avatarClients = struct {
	sync.Mutex
	byProxy map[string]*http.Client
}{byProxy: map[string]*http.Client{}}

// avatarClient returns the client used for proxy, building it once and
// keeping the defaults of http.DefaultTransport (HTTP/2, timeouts, dial
// behavior) apart from the proxy.
func avatarClient(proxy string) *http.Client {
	avatarClients.Lock()
	defer avatarClients.Unlock()

	if client, ok := avatarClients.byProxy[proxy]; ok {
		return client
	}

	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	if proxy != "" {
		if parsed, err := url.Parse(proxy); err == nil {
			transport.Proxy = http.ProxyURL(parsed)
		}
	}

	client := &http.Client{Timeout: 5 * time.Second, Transport: transport}
	avatarClients.byProxy[proxy] = client
	return client
}

// nodeAvatar returns the decoded avatar at url, or nil while it is still
// downloading or when it cannot be loaded. The download starts in the
// background on first use, and the view redraws once it finishes.
func (v *View) nodeAvatar(proxyURL string, url string) ui.ImageSource {
	return avatarImage(proxyURL, url, v.invalidate)
}

// nodeAvatarBitmap returns the avatar as a bitmap, as widgets like
// ui.Avatar take one; SVG pictures and pending or failed loads return nil.
func (v *View) nodeAvatarBitmap(proxyURL string, url string) *ui.Bitmap {
	return avatarBitmap(proxyURL, url, v.invalidate)
}

// avatarImage is the shared loader: invalidate, when set, redraws the
// window that shows the avatar once it is ready.
func avatarImage(proxyURL string, url string, invalidate func()) ui.ImageSource {
	if url == "" {
		return nil
	}

	avatars.Lock()
	if img, ok := avatars.images[url]; ok {
		avatars.Unlock()
		return img
	}
	if avatars.busy[url] {
		avatars.Unlock()
		return nil
	}
	avatars.busy[url] = true
	avatars.Unlock()

	go func() {
		img := downloadAvatar(proxyURL, url)

		avatars.Lock()
		avatars.images[url] = img
		avatars.order = append(avatars.order, url)
		for len(avatars.order) > maxAvatarCache {
			oldest := avatars.order[0]
			avatars.order = avatars.order[1:]
			delete(avatars.images, oldest)
		}
		delete(avatars.busy, url)
		avatars.Unlock()

		if img != nil && invalidate != nil {
			invalidate()
		}
	}()

	return nil
}

// avatarBitmap loads the avatar and keeps it only if it is a bitmap.
func avatarBitmap(proxyURL string, url string, invalidate func()) *ui.Bitmap {
	img, _ := avatarImage(proxyURL, url, invalidate).(*ui.Bitmap)
	return img
}

// downloadAvatar fetches and decodes one picture, or returns nil when the
// request, the response or the decoding fails.
func downloadAvatar(proxyURL string, url string) ui.ImageSource {
	resp, err := avatarClient(proxyURL).Get(url)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20))
	if err != nil {
		return nil
	}

	img, err := ui.DecodeImage(body)
	if err != nil {
		return nil
	}
	return img
}

// resolveImageURL makes a picture address in Markdown absolute against the
// site, for the addresses that come without a scheme.
func resolveImageURL(raw string) string {
	link := strings.TrimSpace(raw)
	switch {
	case strings.HasPrefix(link, "//"):
		return "https:" + link
	case strings.HasPrefix(link, "http://"), strings.HasPrefix(link, "https://"):
		return link
	}
	if !strings.HasPrefix(link, "/") {
		link = "/" + link
	}
	return "https://www.v2ex.com" + link
}

// markdownImages is a markdown.Options LoadImage that fetches through the
// app's proxy, so pictures in a topic's body and its replies show as
// pictures instead of their descriptions.
func markdownImages(proxyURL string) func(string) (ui.ImageSource, error) {
	return func(raw string) (ui.ImageSource, error) {
		link := resolveImageURL(raw)
		img := downloadAvatar(proxyURL, link)
		if img == nil {
			return nil, fmt.Errorf("cannot load image %q", raw)
		}
		return img, nil
	}
}
