package components

import (
	"bytes"
	"fmt"
	"image"
	_ "image/gif"  // looksLikeEmoticon decodes the pictures it classifies
	_ "image/jpeg" // looksLikeEmoticon decodes the pictures it classifies
	_ "image/png"  // looksLikeEmoticon decodes the pictures it classifies
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

// imageUserAgent is a browser's user agent. Picture hosts such as imgur
// answer Go's default one with 429, so the downloads ask as a browser does.
const imageUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) " +
	"AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36"

// avatars caches node avatars by URL. Decoding happens off the UI thread, so
// a row draws its fallback icon until the picture is ready; a failed load is
// cached too, which keeps a broken URL from being retried every frame.
var avatars = struct {
	sync.Mutex
	images map[string]ui.ImageSource
	order  []string
	busy   map[string]bool
	// emoticons names the pictures that look like emojis or stickers, which
	// a reply shows at the size of its words rather than across the pane.
	emoticons map[string]bool
}{images: map[string]ui.ImageSource{}, busy: map[string]bool{}, emoticons: map[string]bool{}}

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
		img, emoticon := downloadAvatar(proxyURL, url)

		avatars.Lock()
		avatars.images[url] = img
		if emoticon {
			avatars.emoticons[url] = true
		}
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

// emoticonPicture reports whether the picture at url looks like an emoji or
// a sticker, which replies draw small beside the text.
func emoticonPicture(url string) bool {
	avatars.Lock()
	defer avatars.Unlock()
	return avatars.emoticons[url]
}

// avatarBitmap loads the avatar and keeps it only if it is a bitmap.
func avatarBitmap(proxyURL string, url string, invalidate func()) *ui.Bitmap {
	img, _ := avatarImage(proxyURL, url, invalidate).(*ui.Bitmap)
	return img
}

// downloadAvatar fetches and decodes one picture, or returns nil when the
// request, the response or the decoding fails. The second result reports
// whether the picture looks like an emoji or sticker.
func downloadAvatar(proxyURL string, url string) (ui.ImageSource, bool) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, false
	}
	req.Header.Set("User-Agent", imageUserAgent)
	req.Header.Set("Accept", "image/*,*/*;q=0.8")

	resp, err := avatarClient(proxyURL).Do(req)
	if err != nil {
		return nil, false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, false
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20))
	if err != nil {
		return nil, false
	}

	img, err := ui.DecodeImage(body)
	if err != nil {
		return nil, false
	}
	return img, looksLikeEmoticon(body)
}

// maxEmoticonSide is the largest side a picture may have and still count as
// an emoticon; a screenshot is usually wider.
const maxEmoticonSide = 1024

// maxEmoticonBytes bounds what looksLikeEmoticon decodes, so a large
// picture costs its header only. The limit also keeps a sticker's likely
// size in mind: an emoji file is small.
const maxEmoticonBytes = 2 << 20

// looksLikeEmoticon reports whether a picture is probably an emoji or a
// sticker: a transparent edge, as those have and screenshots do not, on a
// picture small enough to be one. Only the header is read for a picture too
// large to be an emoticon.
func looksLikeEmoticon(body []byte) bool {
	cfg, _, err := image.DecodeConfig(bytes.NewReader(body))
	if err != nil {
		return false
	}
	if cfg.Width < 16 || cfg.Height < 16 ||
		cfg.Width > maxEmoticonSide || cfg.Height > maxEmoticonSide ||
		len(body) > maxEmoticonBytes {
		return false
	}
	// Emojis are square, or near it; a wide picture with transparency is
	// more likely a logo or a diagram.
	if ratio := float64(cfg.Width) / float64(cfg.Height); ratio < 0.7 || ratio > 1.4 {
		return false
	}

	img, _, err := image.Decode(bytes.NewReader(body))
	if err != nil {
		return false
	}

	// An emoji's round shape leaves its corners and the middle of its top
	// and bottom edges clear; a screenshot paints every one of them.
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	points := [][2]int{
		{0, 0}, {w - 1, 0}, {0, h - 1}, {w - 1, h - 1},
		{w / 2, 0}, {w / 2, h - 1}, {0, h / 2}, {w - 1, h / 2},
	}
	for _, p := range points {
		if _, _, _, alpha := img.At(b.Min.X+p[0], b.Min.Y+p[1]).RGBA(); alpha < 0xffff {
			return true
		}
	}
	return false
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
		img, _ := downloadAvatar(proxyURL, link)
		if img == nil {
			return nil, fmt.Errorf("cannot load image %q", raw)
		}
		return img, nil
	}
}
