package img

import (
	_ "embed"

	"github.com/egoist/mygo/ui"
)

//go:embed image.png
var logoPNG []byte

//go:embed tray.png
var trayPNG []byte

// LogoPNG is the logo's PNG bytes, for places that take a picture rather
// than a decoded source, as a window icon does.
var LogoPNG = logoPNG

// TrayPNG is the logo cropped to its ink and scaled to fill the tray's icon
// slot, so it reads larger there than the padded logo does.
var TrayPNG = trayPNG

// Logo is the decoded logo, for ui.Image.
var Logo ui.ImageSource

func init() {
	src, err := ui.DecodeImage(logoPNG)
	if err != nil {
		panic(err)
	}

	Logo = src
}
