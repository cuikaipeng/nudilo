// Prepares the approved Nudilo artwork and third-party notices for packaging.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"image/png"
	"os"
	"path/filepath"
)

func main() {
	check := flag.Bool("check-licenses", false, "verify reviewed licenses and packaged copies without writing")
	refresh := flag.Bool("refresh-licenses", false, "refresh license copies after reviewing third_party/components.json")
	flag.Parse()
	if *check || *refresh {
		if *check && *refresh {
			panic("choose either -check-licenses or -refresh-licenses")
		}
		mode := "check"
		if *refresh {
			mode = "refresh"
		}
		if err := notices(mode); err != nil {
			panic(err)
		}
		return
	}
	if err := brandAssets(); err != nil {
		panic(err)
	}
	if err := notices("write"); err != nil {
		panic(err)
	}
}

func brandAssets() error {
	if err := os.MkdirAll("resources", 0755); err != nil {
		return err
	}
	// Each surface needs its own framing: a light app tile, the transparent
	// sidebar mark, and tightly framed menu-bar artwork without poster padding.
	assets := []struct {
		source, target string
		square         bool
	}{
		{"touch/nudilo-app-icon.png", "icon.png", true},
		{"touch/nudilo-logo.png", "logo.png", true},
		{"touch/nudilo-tray-template.png", "tray.png", false},
		{"touch/nudilo-tray-color.png", "tray-color.png", false},
	}
	for _, asset := range assets {
		data, err := os.ReadFile(filepath.Join("design", "brand", asset.source))
		if err != nil {
			return err
		}
		config, err := png.DecodeConfig(bytes.NewReader(data))
		if err != nil {
			return fmt.Errorf("decode %s: %w", asset.source, err)
		}
		if config.Width < 256 || config.Height < 256 || (asset.square && config.Width != config.Height) {
			return fmt.Errorf("invalid dimensions for %s: %dx%d", asset.source, config.Width, config.Height)
		}
		if asset.target == "icon.png" {
			// An inset translucent tile can show a second background ring when
			// the OS masks the icon. The app artwork must have an opaque canvas.
			img, err := png.Decode(bytes.NewReader(data))
			if err != nil {
				return fmt.Errorf("decode %s: %w", asset.source, err)
			}
			if opaque, ok := img.(interface{ Opaque() bool }); !ok || !opaque.Opaque() {
				return fmt.Errorf("application icon %s must have a fully opaque background", asset.source)
			}
		}
		if err := os.WriteFile(filepath.Join("resources", asset.target), data, 0644); err != nil {
			return err
		}
	}
	return nil
}
