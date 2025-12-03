package assets

import (
	"bytes"
	"crypto/sha1"
	"encoding/binary"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	imagedraw "golang.org/x/image/draw"
	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
)

// IconSet represents generated icon paths in the formats we support.
type IconSet struct {
	PNG  string
	ICO  string
	ICNS string
}

// EnsureAppIcon renders a deterministic icon using the provided app name and
// caches the PNG/ICO/ICNS variants on disk. The same name always maps to the
// same assets so repeated builds stay stable.
func EnsureAppIcon(appName, baseDir string) (*IconSet, error) {
	if baseDir == "" {
		baseDir = os.TempDir()
	}

	slug := slugify(appName)
	iconDir := filepath.Join(baseDir, ".releaser-icons", slug)
	if err := os.MkdirAll(iconDir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to prepare icon cache: %w", err)
	}

	pngPath := filepath.Join(iconDir, "icon.png")
	icoPath := filepath.Join(iconDir, "icon.ico")
	icnsPath := filepath.Join(iconDir, "icon.icns")

	if _, err := os.Stat(pngPath); errors.Is(err, os.ErrNotExist) {
		if err := renderIconPNG(appName, pngPath); err != nil {
			return nil, err
		}
	}

	pngData, err := os.ReadFile(pngPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read generated icon: %w", err)
	}

	if _, err := os.Stat(icoPath); errors.Is(err, os.ErrNotExist) {
		if err := writeICOFromPNG(pngData, icoPath); err != nil {
			return nil, err
		}
	}

	if _, err := os.Stat(icnsPath); errors.Is(err, os.ErrNotExist) {
		if err := writeICNSFromPNG(pngData, icnsPath); err != nil {
			return nil, err
		}
	}

	return &IconSet{PNG: pngPath, ICO: icoPath, ICNS: icnsPath}, nil
}

func renderIconPNG(appName, target string) error {
	img := image.NewRGBA(image.Rect(0, 0, 256, 256))
	bg := image.NewUniform(colorForName(appName))
	draw.Draw(img, img.Bounds(), bg, image.Point{}, draw.Src)

	initials := deriveInitials(appName)
	textImg := image.NewRGBA(image.Rect(0, 0, 64, 64))
	textDrawer := &font.Drawer{
		Dst:  textImg,
		Src:  image.NewUniform(color.RGBA{255, 255, 255, 255}),
		Face: basicfont.Face7x13,
	}
	textWidth := textDrawer.MeasureString(initials).Ceil()
	textDrawer.Dot = fixed.Point26_6{
		X: fixed.I(max((64-textWidth)/2, 0)),
		Y: fixed.I(38),
	}
	textDrawer.DrawString(initials)

	scaled := image.NewRGBA(img.Bounds())
	imagedraw.NearestNeighbor.Scale(scaled, scaled.Bounds(), textImg, textImg.Bounds(), imagedraw.Over, nil)
	draw.Draw(img, img.Bounds(), scaled, image.Point{}, draw.Over)

	file, err := os.Create(target)
	if err != nil {
		return fmt.Errorf("failed to create icon file: %w", err)
	}
	defer file.Close()

	if err := png.Encode(file, img); err != nil {
		return fmt.Errorf("failed to encode icon png: %w", err)
	}
	return nil
}

func writeICOFromPNG(pngData []byte, target string) error {
	buf := &bytes.Buffer{}
	buf.Grow(6 + 16 + len(pngData))
	buf.Write([]byte{0x00, 0x00, 0x01, 0x00, 0x01, 0x00})
	entry := make([]byte, 16)
	entry[0] = 0                                 // 256px
	entry[1] = 0                                 // 256px
	entry[2] = 0                                 // palette colors
	entry[3] = 0                                 // reserved
	binary.LittleEndian.PutUint16(entry[4:], 1)  // color planes
	binary.LittleEndian.PutUint16(entry[6:], 32) // bits per pixel
	binary.LittleEndian.PutUint32(entry[8:], uint32(len(pngData)))
	binary.LittleEndian.PutUint32(entry[12:], uint32(6+16))
	buf.Write(entry)
	buf.Write(pngData)

	return os.WriteFile(target, buf.Bytes(), 0o644)
}

func writeICNSFromPNG(pngData []byte, target string) error {
	chunkLen := uint32(8 + len(pngData))
	totalLen := uint32(8) + chunkLen
	buf := &bytes.Buffer{}
	buf.Grow(int(totalLen))
	buf.WriteString("icns")
	binary.Write(buf, binary.BigEndian, totalLen)
	buf.WriteString("ic07")
	binary.Write(buf, binary.BigEndian, chunkLen)
	buf.Write(pngData)

	return os.WriteFile(target, buf.Bytes(), 0o644)
}

func deriveInitials(appName string) string {
	name := strings.TrimSpace(appName)
	if name == "" {
		return "APP"
	}
	parts := strings.FieldsFunc(name, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	var initials []rune
	for _, part := range parts {
		letters := []rune(part)
		if len(letters) == 0 {
			continue
		}
		initials = append(initials, unicode.ToUpper(letters[0]))
		if len(initials) == 3 {
			break
		}
	}
	if len(initials) == 0 {
		for _, r := range name {
			if unicode.IsLetter(r) || unicode.IsDigit(r) {
				initials = append(initials, unicode.ToUpper(r))
				if len(initials) == 3 {
					break
				}
			}
		}
	}
	if len(initials) == 0 {
		return "APP"
	}
	return string(initials)
}

func colorForName(appName string) color.RGBA {
	if appName == "" {
		appName = "app"
	}
	sum := sha1.Sum([]byte(strings.ToLower(appName)))
	return color.RGBA{
		R: 80 + sum[0]%120,
		G: 80 + sum[1]%120,
		B: 80 + sum[2]%120,
		A: 255,
	}
}

func slugify(appName string) string {
	lower := strings.ToLower(appName)
	var b strings.Builder
	for _, r := range lower {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			continue
		}
		if b.Len() > 0 && b.String()[b.Len()-1] != '-' {
			b.WriteByte('-')
		}
	}
	slug := strings.Trim(b.String(), "-")
	if slug == "" {
		return "app"
	}
	return slug
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
