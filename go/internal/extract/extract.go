// Package extract loads images, videos, and PDFs into a common sequence of
// searchable "units" (a plain image, a sampled video frame, or a PDF page).
//
// Video decoding and PDF rendering have no good pure-Go equivalents, so this
// package shells out to ffmpeg and poppler-utils (pdftoppm/pdftotext/pdfinfo)
// rather than reimplementing codecs — the same pragmatic tradeoff Python's
// pytesseract makes by wrapping the tesseract CLI instead of an OCR engine.
package extract

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"mediasearch/internal/apperr"
	"mediasearch/internal/exectool"
)

var imageExtensions = map[string]bool{
	".png": true, ".jpg": true, ".jpeg": true, ".gif": true,
	".bmp": true, ".tiff": true, ".tif": true, ".webp": true,
}

var videoExtensions = map[string]bool{
	".mp4": true, ".avi": true, ".mov": true, ".mkv": true,
	".webm": true, ".m4v": true, ".mpg": true, ".mpeg": true,
}

var pdfExtensions = map[string]bool{".pdf": true}

const (
	DefaultFrameIntervalSec = 1.0
	DefaultPDFDPI           = 200
)

// WordBox is one word from a PDF's embedded text layer, in rendered-pixel
// coordinates, with a Line index assigned by extraction-time clustering
// (poppler's -bbox output gives no line grouping of its own).
type WordBox struct {
	Text           string
	X0, Y0, X1, Y1 float64
	Line           int
}

// Unit is one searchable image: a photo, a video frame, or a PDF page.
// Pixel data is loaded lazily from Path (only template matching needs it;
// text search shells out to tesseract against the file directly).
type Unit struct {
	UnitType     string // "image" | "frame" | "page"
	Index        int
	Path         string
	TimestampSec *float64
	// Nil means "no text layer available" (plain image/video, or a scanned
	// PDF page) -- callers should fall back to OCR. Non-nil (even empty)
	// means a PDF text layer was present and should be used as-is.
	TextWords []WordBox
}

// Session holds the extracted units and any temp directory that needs
// cleanup once the caller is done (ffmpeg/pdftoppm output).
type Session struct {
	InputType string
	Units     []Unit
	tempDir   string
}

func (s *Session) Close() error {
	if s.tempDir != "" {
		return os.RemoveAll(s.tempDir)
	}
	return nil
}

type Options struct {
	FrameInterval float64 // video only; <= 0 uses DefaultFrameIntervalSec
	MaxUnits      int     // 0 = no cap
	DPI           int     // pdf only; <= 0 uses DefaultPDFDPI
}

func DetectInputType(path string) (string, error) {
	ext := strings.ToLower(filepath.Ext(path))
	switch {
	case imageExtensions[ext]:
		return "image", nil
	case videoExtensions[ext]:
		return "video", nil
	case pdfExtensions[ext]:
		return "pdf", nil
	}
	return "", apperr.New(
		"unsupported file extension '%s' for '%s'. Supported: images %s, video %s, pdf %s",
		ext, filepath.Base(path), extList(imageExtensions), extList(videoExtensions), extList(pdfExtensions),
	)
}

func extList(m map[string]bool) string {
	exts := make([]string, 0, len(m))
	for e := range m {
		exts = append(exts, e)
	}
	sort.Strings(exts)
	return strings.Join(exts, ", ")
}

// Load validates the path and extension eagerly (failing fast on a bad path
// or unsupported format) and dispatches to the format-specific loader.
func Load(path string, opts Options) (*Session, error) {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return nil, apperr.New("input file not found: %s", path)
	}
	inputType, err := DetectInputType(path)
	if err != nil {
		return nil, err
	}

	switch inputType {
	case "image":
		return &Session{InputType: "image", Units: []Unit{{UnitType: "image", Index: 0, Path: path}}}, nil
	case "video":
		return loadVideo(path, opts)
	case "pdf":
		return loadPDF(path, opts)
	}
	panic("unreachable: unknown input type " + inputType)
}

func loadVideo(path string, opts Options) (*Session, error) {
	interval := opts.FrameInterval
	if interval <= 0 {
		interval = DefaultFrameIntervalSec
	}

	tempDir, err := os.MkdirTemp("", "mediasearch-video-*")
	if err != nil {
		return nil, err
	}

	args := []string{
		"-v", "error",
		"-i", path,
		"-vf", fmt.Sprintf("fps=1/%f", interval),
		"-vsync", "vfr",
	}
	if opts.MaxUnits > 0 {
		args = append(args, "-frames:v", strconv.Itoa(opts.MaxUnits))
	}
	pattern := filepath.Join(tempDir, "frame_%06d.png")
	args = append(args, pattern)

	if _, err := exectool.Run("ffmpeg", args...); err != nil {
		os.RemoveAll(tempDir)
		return nil, fmt.Errorf("could not read video file: %s: %w", path, err)
	}

	files, err := filepath.Glob(filepath.Join(tempDir, "frame_*.png"))
	if err != nil {
		os.RemoveAll(tempDir)
		return nil, err
	}
	sort.Strings(files) // fixed %06d width: lexicographic sort is numeric sort

	units := make([]Unit, len(files))
	for i, f := range files {
		ts := float64(i) * interval
		units[i] = Unit{UnitType: "frame", Index: i, Path: f, TimestampSec: &ts}
	}
	return &Session{InputType: "video", Units: units, tempDir: tempDir}, nil
}

func loadPDF(path string, opts Options) (*Session, error) {
	dpi := opts.DPI
	if dpi <= 0 {
		dpi = DefaultPDFDPI
	}

	pageCount, err := pdfPageCount(path)
	if err != nil {
		return nil, err
	}
	if opts.MaxUnits > 0 && opts.MaxUnits < pageCount {
		pageCount = opts.MaxUnits
	}
	if pageCount == 0 {
		return &Session{InputType: "pdf"}, nil
	}

	tempDir, err := os.MkdirTemp("", "mediasearch-pdf-*")
	if err != nil {
		return nil, err
	}

	if _, err := exectool.Run("pdftoppm",
		"-png", "-r", strconv.Itoa(dpi),
		"-f", "1", "-l", strconv.Itoa(pageCount),
		path, filepath.Join(tempDir, "page"),
	); err != nil {
		os.RemoveAll(tempDir)
		return nil, fmt.Errorf("could not render PDF: %s: %w", path, err)
	}

	pageImages, err := pdftoppmOutputByPage(tempDir, pageCount)
	if err != nil {
		os.RemoveAll(tempDir)
		return nil, err
	}

	wordsByPage, err := pdfTextWords(path, dpi, pageCount)
	if err != nil {
		os.RemoveAll(tempDir)
		return nil, err
	}

	units := make([]Unit, pageCount)
	for i := 0; i < pageCount; i++ {
		units[i] = Unit{
			UnitType:  "page",
			Index:     i,
			Path:      pageImages[i],
			TextWords: wordsByPage[i],
		}
	}
	return &Session{InputType: "pdf", Units: units, tempDir: tempDir}, nil
}

func pdfPageCount(path string) (int, error) {
	out, err := exectool.Run("pdfinfo", path)
	if err != nil {
		return 0, err
	}
	for _, line := range strings.Split(string(out), "\n") {
		if strings.HasPrefix(line, "Pages:") {
			fields := strings.Fields(line)
			if len(fields) == 2 {
				if n, err := strconv.Atoi(fields[1]); err == nil {
					return n, nil
				}
			}
		}
	}
	return 0, apperr.New("could not determine page count for PDF: %s", path)
}

var pdftoppmPageNum = regexp.MustCompile(`page-(\d+)\.png$`)

// pdftoppmOutputByPage maps each 0-indexed page to its rendered file path.
// pdftoppm's numeric suffix width depends on the page range requested, so
// filenames are matched by parsing the embedded page number rather than by
// assuming zero-padding or lexicographic order.
func pdftoppmOutputByPage(tempDir string, pageCount int) ([]string, error) {
	files, err := filepath.Glob(filepath.Join(tempDir, "page-*.png"))
	if err != nil {
		return nil, err
	}
	byPage := make(map[int]string, len(files))
	for _, f := range files {
		m := pdftoppmPageNum.FindStringSubmatch(f)
		if m == nil {
			continue
		}
		n, _ := strconv.Atoi(m[1])
		byPage[n] = f
	}
	result := make([]string, pageCount)
	for i := 0; i < pageCount; i++ {
		path, ok := byPage[i+1] // pdftoppm pages are 1-indexed
		if !ok {
			return nil, apperr.New("pdftoppm did not produce an image for page %d", i+1)
		}
		result[i] = path
	}
	return result, nil
}
