// Command icongen renders the artwork used by the Windows build resources:
// winres/icon.ico (embedded into the executable's resources) and a larger
// winres/icon.png for previews.
//
// The mark is drawn from scratch instead of being checked in as an opaque
// binary blob so that its shape, colours and padding stay reviewable and
// editable: change a constant here, re-run the command, and the icon follows.
// Run it from the backend directory:
//
//	go run ./tools/icongen
//
// The drawing happens in a 1024x1024 design space and is box-filtered down to
// each requested size, which is what keeps the edges smooth at 256px and still
// legible at the 16px size Explorer uses in list views.
package main

import (
	"encoding/binary"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"log"
	"math"
	"os"
	"path/filepath"
)

const (
	canvas   = 1024.0 // design space
	scale    = 4      // supersampling factor per output pixel
	tileEdge = 224.0  // corner radius of the rounded tile
)

// The glyph: a lowercase "b" -- the initial of bs-blog -- built from a vertical
// capsule (the ascender) and an annulus (the bowl). The bowl is centred far
// enough left that it overlaps the stem, so the two read as one letter rather
// than two shapes; its bottom is flush with the stem's bottom.
const (
	stemX0 = 256.0
	stemX1 = 379.0
	stemY0 = 172.0
	stemY1 = 852.0

	bowlCX    = 568.0
	bowlCY    = 630.0
	bowlOuter = 222.0
	bowlInner = 112.0
)

// icoSizes are the resolutions packed into icon.ico. 16/32/48 cover Explorer's
// list, taskbar and desktop sizes, 256 is the shell's "extra large" artwork.
var icoSizes = []int{16, 24, 32, 48, 64, 128, 256}

type rgb struct{ r, g, b float64 }

func rgbOf(c color.NRGBA) rgb {
	return rgb{float64(c.R) / 255, float64(c.G) / 255, float64(c.B) / 255}
}

var (
	// Deep plum fading to near-black: the site's dark-first background.
	tileTop    = rgbOf(color.NRGBA{R: 0x2b, G: 0x17, B: 0x33, A: 0xff})
	tileBottom = rgbOf(color.NRGBA{R: 0x0e, G: 0x0a, B: 0x13, A: 0xff})

	// The site accent, #ec4899, split into a lighter top and darker bottom.
	glyphTop    = rgbOf(color.NRGBA{R: 0xf4, G: 0x72, B: 0xb6, A: 0xff})
	glyphBottom = rgbOf(color.NRGBA{R: 0xdb, G: 0x27, B: 0x77, A: 0xff})
)

func lerp(a, b rgb, t float64) rgb {
	return rgb{
		r: a.r + (b.r-a.r)*t,
		g: a.g + (b.g-a.g)*t,
		b: a.b + (b.b-a.b)*t,
	}
}

// inTile reports whether (x, y) falls inside the rounded square that fills the
// whole canvas.
func inTile(x, y, radius float64) bool {
	if x < 0 || y < 0 || x > canvas || y > canvas {
		return false
	}
	// Clamping to the inner rect turns the rounded-rect test into a
	// distance-to-nearest-point test: outside that rect the corner arcs apply,
	// inside it the distance is zero.
	cx := math.Min(math.Max(x, radius), canvas-radius)
	cy := math.Min(math.Max(y, radius), canvas-radius)
	dx, dy := x-cx, y-cy
	return dx*dx+dy*dy <= radius*radius
}

// inCapsule reports whether (x, y) falls inside the vertical capsule spanning
// x0..x1 and y0..y1, whose caps are half-circles of radius (x1-x0)/2.
func inCapsule(x, y, x0, x1, y0, y1 float64) bool {
	cx := (x0 + x1) / 2
	radius := (x1 - x0) / 2
	// The capsule is the set of points within radius of the vertical segment
	// joining the two cap centres.
	segTop, segBottom := y0+radius, y1-radius
	if segTop > segBottom {
		segTop, segBottom = segBottom, segTop
	}
	cy := math.Min(math.Max(y, segTop), segBottom)
	dx, dy := x-cx, y-cy
	return dx*dx+dy*dy <= radius*radius
}

// inBowl reports whether (x, y) falls inside the ring centred at (cx, cy).
func inBowl(x, y, cx, cy, outer, inner float64) bool {
	dx, dy := x-cx, y-cy
	d2 := dx*dx + dy*dy
	return d2 <= outer*outer && d2 >= inner*inner
}

// sample returns the straight (non-premultiplied) colour and coverage of one
// design-space point.
func sample(x, y float64) (r, g, b, a float64) {
	if !inTile(x, y, tileEdge) {
		return 0, 0, 0, 0
	}
	t := y / canvas
	fill := lerp(tileTop, tileBottom, t)
	if inCapsule(x, y, stemX0, stemX1, stemY0, stemY1) || inBowl(x, y, bowlCX, bowlCY, bowlOuter, bowlInner) {
		fill = lerp(glyphTop, glyphBottom, t)
	}
	return fill.r, fill.g, fill.b, 1
}

func clamp8(v float64) uint8 {
	switch {
	case v <= 0:
		return 0
	case v >= 255:
		return 255
	default:
		return uint8(math.Round(v))
	}
}

// render box-filters the design-space samples down to a size x size image,
// summing in premultiplied space so that partially covered edge pixels blend
// against the tile instead of against black.
func render(size int) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	subpixels := float64(scale * scale)
	edge := float64(size)

	for oy := 0; oy < size; oy++ {
		for ox := 0; ox < size; ox++ {
			var sumR, sumG, sumB, sumA float64
			for sy := 0; sy < scale; sy++ {
				for sx := 0; sx < scale; sx++ {
					// Sample at the centre of each subpixel, mapped from the
					// output pixel grid into the design space.
					x := (float64(ox) + (float64(sx)+0.5)/scale) / edge * canvas
					y := (float64(oy) + (float64(sy)+0.5)/scale) / edge * canvas
					r, g, b, a := sample(x, y)
					sumR += r * a
					sumG += g * a
					sumB += b * a
					sumA += a
				}
			}

			var px color.NRGBA
			if sumA > 0 {
				px = color.NRGBA{
					R: clamp8(sumR / sumA * 255),
					G: clamp8(sumG / sumA * 255),
					B: clamp8(sumB / sumA * 255),
					A: clamp8(sumA / subpixels * 255),
				}
			}
			img.SetNRGBA(ox, oy, px)
		}
	}
	return img
}

func appendLE16(b []byte, v uint16) []byte {
	return binary.LittleEndian.AppendUint16(b, v)
}

func appendLE32(b []byte, v uint32) []byte {
	return binary.LittleEndian.AppendUint32(b, v)
}

// dib encodes one image in the ICO container's bitmap form: a BITMAPINFOHEADER
// followed by a bottom-up BGRA colour bitmap and a 1bpp AND mask.
func dib(img *image.NRGBA) []byte {
	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	maskStride := (w + 31) / 32 * 4

	buf := make([]byte, 0, 40+w*h*4+maskStride*h)
	buf = appendLE32(buf, 40)            // biSize
	buf = appendLE32(buf, uint32(w))     // biWidth
	buf = appendLE32(buf, uint32(h*2))   // biHeight: colour bitmap + mask
	buf = appendLE16(buf, 1)             // biPlanes
	buf = appendLE16(buf, 32)            // biBitCount
	buf = appendLE32(buf, 0)             // biCompression: BI_RGB
	buf = appendLE32(buf, uint32(w*h*4)) // biSizeImage
	buf = appendLE32(buf, 0)             // biXPelsPerMeter
	buf = appendLE32(buf, 0)             // biYPelsPerMeter
	buf = appendLE32(buf, 0)             // biClrUsed
	buf = appendLE32(buf, 0)             // biClrImportant

	for y := h - 1; y >= 0; y-- {
		for x := 0; x < w; x++ {
			c := img.NRGBAAt(bounds.Min.X+x, bounds.Min.Y+y)
			buf = append(buf, c.B, c.G, c.R, c.A)
		}
	}
	// The mask is required even for 32-bit icons, but all-zero is correct:
	// transparency is carried entirely by the alpha channel above.
	buf = append(buf, make([]byte, maskStride*h)...)
	return buf
}

// writeICO packs the images into a Windows icon file.
//
// Every entry is stored as a BMP rather than a PNG, because that is what the
// icon readers in the Windows toolchain accept without argument -- including
// goversioninfo, which decodes the .ico to re-emit it into the .syso.
func writeICO(w io.Writer, imgs []*image.NRGBA) error {
	header := make([]byte, 6+16*len(imgs))
	binary.LittleEndian.PutUint16(header[0:], 0) // reserved
	binary.LittleEndian.PutUint16(header[2:], 1) // type: icon
	binary.LittleEndian.PutUint16(header[4:], uint16(len(imgs)))

	var payload []byte
	offset := len(header)
	for i, img := range imgs {
		data := dib(img)
		entry := header[6+16*i : 6+16*(i+1)]
		entry[0] = iconEdge(img.Bounds().Dx())
		entry[1] = iconEdge(img.Bounds().Dy())
		entry[2] = 0                                 // palette entries
		entry[3] = 0                                 // reserved
		binary.LittleEndian.PutUint16(entry[4:], 1)  // colour planes
		binary.LittleEndian.PutUint16(entry[6:], 32) // bits per pixel
		binary.LittleEndian.PutUint32(entry[8:], uint32(len(data)))
		binary.LittleEndian.PutUint32(entry[12:], uint32(offset))
		offset += len(data)
		payload = append(payload, data...)
	}

	if _, err := w.Write(header); err != nil {
		return err
	}
	_, err := w.Write(payload)
	return err
}

// iconEdge converts a pixel edge to the ICONDIRENTRY byte, where 256 is stored
// as 0 because the field is only eight bits wide.
func iconEdge(px int) byte {
	if px >= 256 {
		return 0
	}
	return byte(px)
}

// writeFile creates path and hands the handle to fn, closing it exactly once.
func writeFile(path string, fn func(io.Writer) error) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	if err := fn(f); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

func main() {
	pngOut := flag.String("o", filepath.Join("winres", "icon.png"), "path of the PNG to write")
	icoOut := flag.String("ico", filepath.Join("winres", "icon.ico"), "path of the ICO to write")
	pngSize := flag.Int("size", 256, "edge length of the PNG, in pixels")
	flag.Parse()

	if *pngSize < 16 || *pngSize > 1024 {
		log.Fatalf("icongen: -size must be between 16 and 1024, got %d", *pngSize)
	}

	// The paths are relative to wherever the caller stands, so say so plainly
	// rather than failing with a bare "no such file or directory".
	for _, path := range []string{*pngOut, *icoOut} {
		dir := filepath.Dir(path)
		if _, err := os.Stat(dir); err != nil {
			log.Fatalf("icongen: output directory %q is not usable: %v\n"+
				"run this from the backend directory, or pass -o/-ico an existing path", dir, err)
		}
	}

	if err := writeFile(*pngOut, func(w io.Writer) error {
		return png.Encode(w, render(*pngSize))
	}); err != nil {
		log.Fatalf("icongen: encode %s: %v", *pngOut, err)
	}

	imgs := make([]*image.NRGBA, 0, len(icoSizes))
	for _, size := range icoSizes {
		imgs = append(imgs, render(size))
	}
	if err := writeFile(*icoOut, func(w io.Writer) error {
		return writeICO(w, imgs)
	}); err != nil {
		log.Fatalf("icongen: encode %s: %v", *icoOut, err)
	}

	for _, path := range []string{*pngOut, *icoOut} {
		info, err := os.Stat(path)
		if err != nil {
			log.Fatalf("icongen: stat %s: %v", path, err)
		}
		fmt.Printf("wrote %s (%d bytes)\n", path, info.Size())
	}
}
