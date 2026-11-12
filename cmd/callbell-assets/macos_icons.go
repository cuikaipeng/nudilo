package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"

	"golang.org/x/image/draw"
)

// MyGo v0.7.4 copies AppIcon.icns to the DMG's .VolumeIcon.icns. Give it
// the inset, rounded volume artwork via macos.icon, while CFBundleIconFile
// points the application to Nudilo.icns, made from its opaque original.
// Both files are prepared on every host so cross-builds use the same inputs.
func macOSIcons() error {
	dir := filepath.Join("resources", "darwin")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	for _, icon := range []struct{ source, target string }{
		{"nudilo-app-icon.png", "Nudilo.icns"},
		{"nudilo-volume-icon.png", "VolumeIcon.icns"},
	} {
		data, err := os.ReadFile(filepath.Join("design", "brand", "touch", icon.source))
		if err != nil {
			return err
		}
		img, err := png.Decode(bytes.NewReader(data))
		if err != nil {
			return fmt.Errorf("decode %s: %w", icon.source, err)
		}
		if img.Bounds().Dx() != img.Bounds().Dy() || img.Bounds().Dx() < 1024 {
			return fmt.Errorf("macOS icon %s must be square and at least 1024px", icon.source)
		}
		data, err = encodeICNS(img)
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(dir, icon.target), data, 0644); err != nil {
			return err
		}
	}
	return nil
}

func encodeICNS(src image.Image) ([]byte, error) {
	var body bytes.Buffer
	// Use PNG-compatible ICNS types. macOS reads icp4/icp5 as legacy ARGB;
	// the small Finder sizes should scale down from ic11/ic12 instead.
	for _, entry := range []struct {
		kind string
		size int
	}{
		{"ic11", 32}, {"ic12", 64}, {"ic07", 128}, {"ic13", 256},
		{"ic08", 256}, {"ic14", 512}, {"ic09", 512}, {"ic10", 1024},
	} {
		img := image.NewNRGBA(image.Rect(0, 0, entry.size, entry.size))
		draw.CatmullRom.Scale(img, img.Bounds(), src, src.Bounds(), draw.Src, nil)
		var encoded bytes.Buffer
		if err := png.Encode(&encoded, img); err != nil {
			return nil, err
		}
		body.WriteString(entry.kind)
		_ = binary.Write(&body, binary.BigEndian, uint32(encoded.Len()+8))
		body.Write(encoded.Bytes())
	}
	var out bytes.Buffer
	out.WriteString("icns")
	_ = binary.Write(&out, binary.BigEndian, uint32(body.Len()+8))
	out.Write(body.Bytes())
	return out.Bytes(), nil
}
