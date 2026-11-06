package main

import (
	"log"
	"os"
	"strings"

	"mygo_demo/assets/img"
	"mygo_demo/internal/api"
	"mygo_demo/internal/database"
	"mygo_demo/internal/database/repo"
	"mygo_demo/internal/ui/components"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// defaultProxy is the local proxy used when neither the saved settings nor
// the environment name one.
const defaultProxy = "http://127.0.0.1:7890"

// proxyURL returns the proxy API requests go through. A value saved in the
// app's settings wins, so the settings panel can change it; otherwise
// HTTPS_PROXY wins over HTTP_PROXY because every V2EX endpoint is https,
// and an empty variable counts as unset.
func proxyURL(settings *repo.Settings) string {
	if stored, ok, err := settings.Get(repo.ProxyKey); err == nil && ok {
		return stored
	}

	for _, key := range []string{"HTTPS_PROXY", "HTTP_PROXY"} {
		if value := strings.TrimSpace(os.Getenv(key)); value != "" {
			return value
		}
	}
	return defaultProxy
}

func main() {
	conn, err := database.Init()
	if err != nil {
		log.Fatal(err)
	}
	settings := repo.NewSettings(conn)

	proxy := proxyURL(settings)

	// A token saved from the settings panel wins over the environment, so
	// the app can be set up without editing .env by hand.
	savedToken, hasToken, tokenErr := settings.Get(repo.TokenKey)
	var client *api.Client
	if tokenErr == nil && hasToken && savedToken != "" {
		client, err = api.NewClientWithToken(savedToken, proxy)
	} else {
		client, err = api.NewClient(proxy)
	}
	if err != nil {
		log.Fatal(err)
	}

	cache, err := api.OpenCache(api.DefaultCachePath())
	if err != nil {
		// The cache is an optimization: without it the app still works,
		// it just refetches the listings.
		log.Printf("cache disabled: %v", err)
	} else {
		defer cache.Close()
		client.Cache = cache
	}

	view := components.NewView(client, proxy)

	mygo.App.WhenReady(func() {
		win := mygo.NewWindow(mygo.WindowOptions{
			Title:  "V2EX",
			Width:  1100,
			Height: 760,

			MinWidth:          960,
			MinHeight:         640,
			Content:           ui.View(view.Draw),
			TitleBarStyle:     mygo.TitleBarHidden,
			HideWindowButtons: true,
			BackgroundColor:   "light-dark(#f5f6f8, #16181c)",
		})

		// WindowOptions has no icon field; the window takes one after it is
		// created, which is what the taskbar and the window's own chrome show.
		if err := win.SetIcon(img.LogoPNG); err != nil {
			log.Printf("window icon: %v", err)
		}
		view.Win = win
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}
