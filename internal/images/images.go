// Package images decodes, re-encodes and stores recipe photos. Every image is
// fully re-encoded, which strips metadata (location, camera) and neutralises
// anything hiding in the original file.
package images

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	"image/color"
	_ "image/gif" // register decoders
	"image/jpeg"
	_ "image/png"
	"os"
	"path/filepath"
	"regexp"

	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp"
)

const (
	maxPixels = 60_000_000
	fullSize  = 1600
	thumbSize = 640
)

// Store writes images below a directory as <key>.jpg and <key>_t.jpg.
type Store struct{ Dir string }

var keyRe = regexp.MustCompile(`^[a-f0-9]{24}(_t)?\.jpg$`)

// ValidName reports whether name is a file this store could have written.
func ValidName(name string) bool { return keyRe.MatchString(name) }

// Save decodes data, writes a full-size and a thumbnail JPEG, and returns the
// image key plus the average colour (used as a placeholder while loading).
func (s *Store) Save(data []byte) (key, avg string, err error) {
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return "", "", errors.New("that file isn't an image Fond can read (JPEG, PNG, WebP or GIF)")
	}
	if cfg.Width*cfg.Height > maxPixels || cfg.Width < 8 || cfg.Height < 8 {
		return "", "", errors.New("image dimensions are out of range")
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return "", "", fmt.Errorf("could not decode image: %w", err)
	}
	img = orient(img, exifOrientation(data))

	raw := make([]byte, 12)
	if _, err := rand.Read(raw); err != nil {
		return "", "", err
	}
	key = hex.EncodeToString(raw)
	if err := os.MkdirAll(s.Dir, 0o755); err != nil {
		return "", "", err
	}
	full := resize(img, fullSize)
	if err := writeJPEG(filepath.Join(s.Dir, key+".jpg"), full, 84); err != nil {
		return "", "", err
	}
	thumb := resize(full, thumbSize)
	if err := writeJPEG(filepath.Join(s.Dir, key+"_t.jpg"), thumb, 80); err != nil {
		return "", "", err
	}
	return key, average(thumb), nil
}

// Delete removes both renditions of an image.
func (s *Store) Delete(key string) {
	if !ValidName(key + ".jpg") {
		return
	}
	os.Remove(filepath.Join(s.Dir, key+".jpg"))
	os.Remove(filepath.Join(s.Dir, key+"_t.jpg"))
}

func writeJPEG(path string, img image.Image, q int) error {
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: q}); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, buf.Bytes(), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func resize(src image.Image, maxSide int) image.Image {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	if w <= maxSide && h <= maxSide {
		// Flatten onto white so transparent PNGs don't turn black as JPEG.
		dst := image.NewRGBA(image.Rect(0, 0, w, h))
		draw.Draw(dst, dst.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)
		draw.Draw(dst, dst.Bounds(), src, b.Min, draw.Over)
		return dst
	}
	nw, nh := maxSide, h*maxSide/w
	if h > w {
		nw, nh = w*maxSide/h, maxSide
	}
	dst := image.NewRGBA(image.Rect(0, 0, max(nw, 1), max(nh, 1)))
	draw.Draw(dst, dst.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)
	draw.CatmullRom.Scale(dst, dst.Bounds(), src, b, draw.Over, nil)
	return dst
}

func average(img image.Image) string {
	b := img.Bounds()
	var r, g, bl, n uint64
	for y := b.Min.Y; y < b.Max.Y; y += 4 {
		for x := b.Min.X; x < b.Max.X; x += 4 {
			cr, cg, cb, _ := img.At(x, y).RGBA()
			r, g, bl, n = r+uint64(cr>>8), g+uint64(cg>>8), bl+uint64(cb>>8), n+1
		}
	}
	if n == 0 {
		return ""
	}
	return fmt.Sprintf("#%02x%02x%02x", r/n, g/n, bl/n)
}

// exifOrientation reads the EXIF orientation tag (1-8) from a JPEG, or 1.
func exifOrientation(data []byte) int {
	if len(data) < 4 || data[0] != 0xFF || data[1] != 0xD8 {
		return 1
	}
	i := 2
	for i+4 <= len(data) {
		if data[i] != 0xFF {
			return 1
		}
		marker := data[i+1]
		size := int(binary.BigEndian.Uint16(data[i+2:]))
		if marker == 0xDA || size < 2 {
			return 1
		}
		if marker == 0xE1 && i+4+size-2 <= len(data) {
			seg := data[i+4 : i+2+size]
			if len(seg) > 14 && string(seg[:6]) == "Exif\x00\x00" {
				return tiffOrientation(seg[6:])
			}
		}
		i += 2 + size
	}
	return 1
}

func tiffOrientation(t []byte) int {
	if len(t) < 8 {
		return 1
	}
	var bo binary.ByteOrder = binary.BigEndian
	if string(t[:2]) == "II" {
		bo = binary.LittleEndian
	}
	off := int(bo.Uint32(t[4:]))
	if off+2 > len(t) {
		return 1
	}
	n := int(bo.Uint16(t[off:]))
	for k := 0; k < n; k++ {
		e := off + 2 + k*12
		if e+12 > len(t) {
			return 1
		}
		if bo.Uint16(t[e:]) == 0x0112 {
			if v := int(bo.Uint16(t[e+8:])); v >= 1 && v <= 8 {
				return v
			}
			return 1
		}
	}
	return 1
}

func orient(src image.Image, o int) image.Image {
	if o <= 1 || o > 8 {
		return src
	}
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	dw, dh := w, h
	if o >= 5 {
		dw, dh = h, w
	}
	dst := image.NewRGBA(image.Rect(0, 0, dw, dh))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			var dx, dy int
			switch o {
			case 2:
				dx, dy = w-1-x, y
			case 3:
				dx, dy = w-1-x, h-1-y
			case 4:
				dx, dy = x, h-1-y
			case 5:
				dx, dy = y, x
			case 6:
				dx, dy = h-1-y, x
			case 7:
				dx, dy = h-1-y, w-1-x
			case 8:
				dx, dy = y, w-1-x
			}
			dst.Set(dx, dy, src.At(b.Min.X+x, b.Min.Y+y))
		}
	}
	return dst
}
