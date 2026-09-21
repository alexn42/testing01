package imagematch

import (
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"os"

	_ "golang.org/x/image/bmp"
	_ "golang.org/x/image/tiff"
	_ "golang.org/x/image/webp"
)

// rgbImage is a simple row-major RGB pixel buffer (3 bytes/pixel). Used
// instead of Go's image.Image interface for fast indexed access in the
// matching loops -- At(x,y) through the interface is too slow there.
type rgbImage struct {
	w, h int
	pix  []uint8
}

type grayImage struct {
	w, h int
	pix  []uint8
}

func decodeRGB(path string) (*rgbImage, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("could not decode image: %s: %w", path, err)
	}
	return toRGBImage(img), nil
}

func toRGBImage(img image.Image) *rgbImage {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	out := &rgbImage{w: w, h: h, pix: make([]uint8, w*h*3)}
	i := 0
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			r, g, bl, _ := img.At(x, y).RGBA()
			out.pix[i] = uint8(r >> 8)
			out.pix[i+1] = uint8(g >> 8)
			out.pix[i+2] = uint8(bl >> 8)
			i += 3
		}
	}
	return out
}

// gray converts to grayscale using BT.601 luma weights (matching OpenCV's
// default, for behavioral parity with the Python version).
func (im *rgbImage) gray() *grayImage {
	n := im.w * im.h
	out := &grayImage{w: im.w, h: im.h, pix: make([]uint8, n)}
	for i := 0; i < n; i++ {
		r := float64(im.pix[i*3])
		g := float64(im.pix[i*3+1])
		b := float64(im.pix[i*3+2])
		out.pix[i] = uint8(0.299*r + 0.587*g + 0.114*b)
	}
	return out
}

func (im *rgbImage) crop(x, y, w, h int) *rgbImage {
	out := &rgbImage{w: w, h: h, pix: make([]uint8, w*h*3)}
	for row := 0; row < h; row++ {
		srcOff := ((y+row)*im.w + x) * 3
		dstOff := row * w * 3
		copy(out.pix[dstOff:dstOff+w*3], im.pix[srcOff:srcOff+w*3])
	}
	return out
}

// resizeNearest is adequate for coarse localization and template rescaling;
// it is not intended to produce a visually smooth result.
func (im *rgbImage) resizeNearest(w, h int) *rgbImage {
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	out := &rgbImage{w: w, h: h, pix: make([]uint8, w*h*3)}
	for y := 0; y < h; y++ {
		sy := min(y*im.h/h, im.h-1)
		for x := 0; x < w; x++ {
			sx := min(x*im.w/w, im.w-1)
			si := (sy*im.w + sx) * 3
			di := (y*w + x) * 3
			out.pix[di], out.pix[di+1], out.pix[di+2] = im.pix[si], im.pix[si+1], im.pix[si+2]
		}
	}
	return out
}

func (im *grayImage) resizeNearest(w, h int) *grayImage {
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	out := &grayImage{w: w, h: h, pix: make([]uint8, w*h)}
	for y := 0; y < h; y++ {
		sy := min(y*im.h/h, im.h-1)
		for x := 0; x < w; x++ {
			sx := min(x*im.w/w, im.w-1)
			out.pix[y*w+x] = im.pix[sy*im.w+sx]
		}
	}
	return out
}

// colorSimilarity is 1 - normalized mean absolute pixel difference (1 =
// identical, 0 = maximally different). a and b must be the same size.
func colorSimilarity(a, b *rgbImage) float64 {
	n := len(a.pix)
	if n == 0 || n != len(b.pix) {
		return 0
	}
	var sum int
	for i := 0; i < n; i++ {
		d := int(a.pix[i]) - int(b.pix[i])
		if d < 0 {
			d = -d
		}
		sum += d
	}
	return 1.0 - float64(sum)/float64(n)/255.0
}
