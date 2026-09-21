// Package testfixture builds synthetic images/video used by other
// packages' tests: known text on a plain background (OCR-readable, via
// golang.org/x/image/font/basicfont), and a scene with a template crop.
//
// A PDF can't be generated without a dependency Go's ecosystem doesn't
// have a good answer for, so testdata/sample.pdf is a small, static,
// committed fixture instead (2 pages of real embedded text, 1 page that is
// only a pasted image with no text layer -- forcing the OCR fallback path).
package testfixture

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"

	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
)

// TextImage writes a PNG containing the given text on a white background
// and returns its path.
func TextImage(t *testing.T, dir, text string) string {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 600, 200))
	fillRect(img, 0, 0, 600, 200, color.White)
	DrawText(img, 30, 100, text)
	path := filepath.Join(dir, "text_sample.png")
	writePNG(t, path, img)
	return path
}

// DrawText draws text at (x,y) (baseline origin, matching font.Drawer's Dot
// convention) using a built-in bitmap font -- crude to look at, but reliably
// OCR-readable, which is all fixtures need.
func DrawText(img draw.Image, x, y int, text string) {
	d := &font.Drawer{
		Dst:  img,
		Src:  image.NewUniform(color.Black),
		Face: basicfont.Face7x13,
		Dot:  fixed.Point26_6{X: fixed.I(x), Y: fixed.I(y)},
	}
	d.DrawString(text)
}

type BBox struct{ X, Y, Width, Height int }

type Scene struct {
	ScenePath            string
	TemplatePath         string
	TemplateBBox         BBox
	NegativeTemplatePath string // flat solid color: never present in the scene
	TexturedAbsentPath   string // checkerboard: textured, but never present either
}

// BuildScene draws a few solid-colored shapes on a canvas, crops one of
// them out as a template (guaranteed to match by construction, since it's
// pixel-identical to the region it came from), and also produces two
// deliberately-absent templates: a flat color (the degenerate case for a
// naive correlation-based matcher) and a textured-but-absent checkerboard.
func BuildScene(t *testing.T, dir string) Scene {
	t.Helper()
	canvas := image.NewRGBA(image.Rect(0, 0, 800, 600))
	fillRect(canvas, 0, 0, 800, 600, color.White)
	fillRect(canvas, 50, 50, 200, 150, color.RGBA{R: 255, A: 255})
	fillEllipse(canvas, 300, 200, 480, 380, color.RGBA{B: 255, A: 255})
	fillRect(canvas, 600, 100, 700, 250, color.RGBA{G: 255, A: 255})

	scenePath := filepath.Join(dir, "scene.png")
	writePNG(t, scenePath, canvas)

	templateRect := image.Rect(300, 200, 480, 380)
	template := image.NewRGBA(image.Rect(0, 0, templateRect.Dx(), templateRect.Dy()))
	draw.Draw(template, template.Bounds(), canvas, templateRect.Min, draw.Src)
	templatePath := filepath.Join(dir, "template_blue_ellipse.png")
	writePNG(t, templatePath, template)

	negative := image.NewRGBA(image.Rect(0, 0, 150, 150))
	fillRect(negative, 0, 0, 150, 150, color.RGBA{R: 255, G: 255, A: 255})
	negativePath := filepath.Join(dir, "template_negative.png")
	writePNG(t, negativePath, negative)

	checker := image.NewRGBA(image.Rect(0, 0, 60, 60))
	fillCheckerboard(checker, 10)
	checkerPath := filepath.Join(dir, "template_checkerboard.png")
	writePNG(t, checkerPath, checker)

	return Scene{
		ScenePath:            scenePath,
		TemplatePath:         templatePath,
		TemplateBBox:         BBox{300, 200, templateRect.Dx(), templateRect.Dy()},
		NegativeTemplatePath: negativePath,
		TexturedAbsentPath:   checkerPath,
	}
}

// BuildVideo encodes blankFrames frames of a plain background followed by
// textedFrames frames with text, at the given fps, via ffmpeg (already a
// hard runtime dependency of the tool, so this adds nothing new).
func BuildVideo(t *testing.T, dir string, fps, blankFrames, textedFrames int, text string) string {
	t.Helper()
	frameDir := filepath.Join(dir, "frames")
	if err := os.MkdirAll(frameDir, 0o755); err != nil {
		t.Fatal(err)
	}

	blank := image.NewRGBA(image.Rect(0, 0, 640, 240))
	fillRect(blank, 0, 0, 640, 240, color.White)

	texted := image.NewRGBA(image.Rect(0, 0, 640, 240))
	fillRect(texted, 0, 0, 640, 240, color.White)
	DrawText(texted, 40, 130, text)

	total := blankFrames + textedFrames
	for i := 0; i < total; i++ {
		frame := blank
		if i >= blankFrames {
			frame = texted
		}
		writePNG(t, filepath.Join(frameDir, fmt.Sprintf("f_%06d.png", i)), frame)
	}

	videoPath := filepath.Join(dir, "sample.mp4")
	cmd := exec.Command("ffmpeg", "-y", "-v", "error",
		"-framerate", strconv.Itoa(fps),
		"-i", filepath.Join(frameDir, "f_%06d.png"),
		"-pix_fmt", "yuv420p",
		videoPath,
	)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("ffmpeg encode failed: %v: %s", err, stderr.String())
	}
	return videoPath
}

// SamplePDFPath returns the path to the committed static PDF fixture
// (testdata/sample.pdf, 3 pages: 2 with a digital text layer, 1 image-only).
func SamplePDFPath(t *testing.T) string {
	t.Helper()
	path, err := filepath.Abs(filepath.Join("..", "..", "testdata", "sample.pdf"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("testdata/sample.pdf not found at %s: %v", path, err)
	}
	return path
}

func fillRect(img draw.Image, x0, y0, x1, y1 int, c color.Color) {
	draw.Draw(img, image.Rect(x0, y0, x1, y1), image.NewUniform(c), image.Point{}, draw.Src)
}

func fillEllipse(img draw.Image, x0, y0, x1, y1 int, c color.Color) {
	cx, cy := float64(x0+x1)/2, float64(y0+y1)/2
	rx, ry := float64(x1-x0)/2, float64(y1-y0)/2
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			dx := (float64(x) + 0.5 - cx) / rx
			dy := (float64(y) + 0.5 - cy) / ry
			if dx*dx+dy*dy <= 1.0 {
				img.Set(x, y, c)
			}
		}
	}
}

func fillCheckerboard(img draw.Image, cell int) {
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			c := color.RGBA{R: 255, G: 255, B: 255, A: 255}
			if ((x-b.Min.X)/cell+(y-b.Min.Y)/cell)%2 == 0 {
				c = color.RGBA{A: 255}
			}
			img.Set(x, y, c)
		}
	}
}

func writePNG(t *testing.T, path string, img image.Image) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
}
