// Command genassets deterministically renders ReelPing's raster brand assets
// (PNG icons, favicon.ico, social card) from the canonical vector geometry.
//
// It reproduces the same mark defined in assets/reelping.svg using a small
// supersampled shape rasteriser (no external SVG renderer required), so the
// output is byte-stable across runs. Run it with:
//
//	go run ./tools/genassets
package main

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"
	"os"
	"path/filepath"

	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

var (
	teal  = color.RGBA{0x0f, 0x9d, 0x9d, 0xff}
	teal2 = color.RGBA{0x17, 0xb8, 0xb8, 0xff}
	green = color.RGBA{0x2e, 0xcc, 0x71, 0xff}
	white = color.RGBA{0xff, 0xff, 0xff, 0xff}
	dark  = color.RGBA{0x12, 0x16, 0x1c, 0xff}
)

// hole positions in the 64-unit design space.
var holes = [][2]float64{
	{25, 22}, {35.4, 28}, {35.4, 40}, {25, 46}, {14.6, 40}, {14.6, 28},
}

const (
	cx, cy = 25.0, 34.0
	ringR  = 17.0
	ringHW = 2.0 // half stroke width
	hubR   = 4.4
	holeR  = 2.9
	dotX   = 49.0
	dotY   = 45.0
	dotR   = 6.0
	dotIn  = 4.4
)

// sample returns the premultiplied colour of the mark at design-space (x,y),
// or transparent. bg is the background used for the dot's border ring.
func sample(x, y float64, borderColor color.RGBA) (color.RGBA, bool) {
	// Dot (front-most): green core with a border ring.
	dd := math.Hypot(x-dotX, y-dotY)
	if dd <= dotIn {
		return green, true
	}
	if dd <= dotR {
		return borderColor, true
	}
	// Signal waves: two concentric arcs to the upper-right of the reel.
	for _, r := range []float64{22.0, 27.0} {
		d := math.Hypot(x-cx, y-cy)
		if math.Abs(d-r) <= 2.0 {
			ang := math.Atan2(y-cy, x-cx) * 180 / math.Pi
			if ang >= -85 && ang <= 18 {
				return teal2, true
			}
		}
	}
	// Reel ring.
	d := math.Hypot(x-cx, y-cy)
	if math.Abs(d-ringR) <= ringHW {
		return teal, true
	}
	// Hub.
	if d <= hubR {
		return teal, true
	}
	// Perforations.
	for _, h := range holes {
		if math.Hypot(x-h[0], y-h[1]) <= holeR {
			return teal, true
		}
	}
	return color.RGBA{}, false
}

// renderMark rasterises the mark at the given pixel size with supersampling.
func renderMark(size int, bg color.RGBA, transparent bool) *image.RGBA {
	const ss = 4
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	if !transparent {
		draw.Draw(img, img.Bounds(), &image.Uniform{bg}, image.Point{}, draw.Src)
	}
	scale := 64.0 / float64(size)
	borderColor := bg
	if transparent {
		borderColor = white
	}
	for py := 0; py < size; py++ {
		for px := 0; px < size; px++ {
			var r, g, b, a float64
			for sy := 0; sy < ss; sy++ {
				for sx := 0; sx < ss; sx++ {
					dx := (float64(px) + (float64(sx)+0.5)/ss) * scale
					dy := (float64(py) + (float64(sy)+0.5)/ss) * scale
					if c, ok := sample(dx, dy, borderColor); ok {
						r += float64(c.R)
						g += float64(c.G)
						b += float64(c.B)
						a += 255
					}
				}
			}
			n := float64(ss * ss)
			cov := a / (255 * n)
			if cov == 0 {
				if transparent {
					continue
				}
				img.SetRGBA(px, py, bg)
				continue
			}
			fr := uint8(r / a * 255)
			fg := uint8(g / a * 255)
			fb := uint8(b / a * 255)
			if transparent {
				img.SetRGBA(px, py, color.RGBA{fr, fg, fb, uint8(cov * 255)})
			} else {
				// Composite over bg.
				br := float64(bg.R)*(1-cov) + float64(fr)*cov
				bgc := float64(bg.G)*(1-cov) + float64(fg)*cov
				bb := float64(bg.B)*(1-cov) + float64(fb)*cov
				img.SetRGBA(px, py, color.RGBA{uint8(br), uint8(bgc), uint8(bb), 255})
			}
		}
	}
	return img
}

func writePNG(path string, img image.Image) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := png.Encoder{CompressionLevel: png.BestCompression}
	return enc.Encode(f, img)
}

// writeICO writes an ICO container holding PNG-encoded images.
func writeICO(path string, imgs []*image.RGBA) error {
	var buf bytes.Buffer
	// ICONDIR
	binary.Write(&buf, binary.LittleEndian, uint16(0)) // reserved
	binary.Write(&buf, binary.LittleEndian, uint16(1)) // type: icon
	binary.Write(&buf, binary.LittleEndian, uint16(len(imgs)))

	type entry struct {
		data []byte
		w, h int
	}
	var entries []entry
	offset := 6 + 16*len(imgs)
	var body bytes.Buffer
	for _, im := range imgs {
		var pb bytes.Buffer
		if err := png.Encode(&pb, im); err != nil {
			return err
		}
		data := pb.Bytes()
		w := im.Bounds().Dx()
		h := im.Bounds().Dy()
		bw, bh := byte(w), byte(h)
		if w >= 256 {
			bw = 0
		}
		if h >= 256 {
			bh = 0
		}
		buf.WriteByte(bw)
		buf.WriteByte(bh)
		buf.WriteByte(0)                                    // colors
		buf.WriteByte(0)                                    // reserved
		binary.Write(&buf, binary.LittleEndian, uint16(1))  // planes
		binary.Write(&buf, binary.LittleEndian, uint16(32)) // bpp
		binary.Write(&buf, binary.LittleEndian, uint32(len(data)))
		binary.Write(&buf, binary.LittleEndian, uint32(offset))
		offset += len(data)
		body.Write(data)
		entries = append(entries, entry{data, w, h})
	}
	buf.Write(body.Bytes())
	return os.WriteFile(path, buf.Bytes(), 0o644)
}

func loadFace(ttf []byte, size float64) font.Face {
	f, _ := opentype.Parse(ttf)
	face, _ := opentype.NewFace(f, &opentype.FaceOptions{Size: size, DPI: 72, Hinting: font.HintingFull})
	return face
}

func drawText(dst *image.RGBA, face font.Face, x, y int, s string, col color.RGBA) {
	d := &font.Drawer{Dst: dst, Src: &image.Uniform{col}, Face: face, Dot: fixed.P(x, y)}
	d.DrawString(s)
}

func renderSocialCard() *image.RGBA {
	const w, h = 1280, 640
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.Draw(img, img.Bounds(), &image.Uniform{dark}, image.Point{}, draw.Src)

	// Accent bar.
	draw.Draw(img, image.Rect(0, h-12, w, h), &image.Uniform{teal}, image.Point{}, draw.Src)

	// Mark on the left.
	mark := renderMark(360, dark, true)
	draw.Draw(img, image.Rect(90, 140, 90+360, 140+360), mark, image.Point{}, draw.Over)

	// Wordmark + tagline.
	drawText(img, loadFace(gobold.TTF, 132), 520, 300, "ReelPing", white)
	drawText(img, loadFace(goregular.TTF, 46), 524, 372, "Keep your viewers in the loop.", teal2)
	drawText(img, loadFace(goregular.TTF, 30), 524, 452, "Self-hosted Plex status monitor & Discord notifier", color.RGBA{0x9a, 0xa7, 0xb8, 0xff})
	return img
}

func main() {
	out := "assets"
	if len(os.Args) > 1 {
		out = os.Args[1]
	}
	if err := os.MkdirAll(out, 0o755); err != nil {
		panic(err)
	}
	for _, size := range []int{64, 128, 256, 512} {
		img := renderMark(size, dark, true)
		must(writePNG(filepath.Join(out, sizeName(size)), img))
	}
	// favicon.ico (16 + 32, dark rounded background style via transparent mark on dark tile).
	ico16 := tile(renderMark(16, dark, true))
	ico32 := tile(renderMark(32, dark, true))
	must(writeICO(filepath.Join(out, "favicon.ico"), []*image.RGBA{ico16, ico32}))

	must(writePNG(filepath.Join(out, "reelping-social-card.png"), renderSocialCard()))
	// Discord avatar: 512px mark on a solid dark tile (looks good cropped to a
	// circle, unlike a transparent PNG on Discord's grey).
	must(writePNG(filepath.Join(out, "reelping-avatar.png"), renderMark(512, dark, false)))
	println("assets written to", out)
}

// tile composites the transparent mark onto a dark rounded tile for the ICO.
func tile(mark *image.RGBA) *image.RGBA {
	b := mark.Bounds()
	img := image.NewRGBA(b)
	draw.Draw(img, b, &image.Uniform{dark}, image.Point{}, draw.Src)
	draw.Draw(img, b, mark, image.Point{}, draw.Over)
	return img
}

func sizeName(s int) string {
	switch s {
	case 64:
		return "reelping-64.png"
	case 128:
		return "reelping-128.png"
	case 256:
		return "reelping-256.png"
	default:
		return "reelping-512.png"
	}
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}
