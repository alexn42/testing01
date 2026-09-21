# mediasearch (Go)

A Go port of the [Python `mediasearch` tool](../README.md): a command-line
program that searches an image, video, or PDF for either a text string or a
template image, and reports the result as JSON — whether it was found, and
where. Same behavior and the same JSON schema as the Python version, so
either can be swapped in for the other.

Compiles to a single, dependency-free (no cgo) binary, which is what makes
cross-compiling a native Windows `.exe` from any platform trivial — see
**Build** below.

## Why a second implementation instead of bindings

Go has no OpenCV-equivalent, production-quality OCR engine, or general video
codec support in pure Go. Rather than reaching for cgo bindings to OpenCV/
Tesseract (which would defeat the point of a portable, easy-to-cross-compile
binary), this port:

- **Shells out** to `ffmpeg` (video frame sampling) and `poppler-utils`
  (`pdftoppm`/`pdftotext`/`pdfinfo` — PDF rendering and text-layer
  extraction) and `tesseract` (OCR) — the same pragmatic choice Python's own
  `pytesseract` makes by wrapping the `tesseract` CLI instead of linking an
  OCR engine.
- **Implements template (image) matching natively** in Go, since that's
  genuinely tractable without a vision library. See **How image matching
  differs from the Python version** below — this isn't just a port of the
  same algorithm, because the same algorithm would need OpenCV.

## Install

Runtime dependencies (none are needed to *build* the binary, only to *run*
it — same idea as the Python version needing `tesseract-ocr` installed):

```bash
apt-get install ffmpeg poppler-utils tesseract-ocr   # Debian/Ubuntu
brew install ffmpeg poppler tesseract                # macOS
```

On Windows, install [ffmpeg](https://ffmpeg.org/download.html),
[poppler for Windows](https://github.com/oschwartz10612/poppler-windows),
and [Tesseract for Windows](https://github.com/UB-Mannheim/tesseract/wiki),
and put each on `PATH`.

For a step-by-step walkthrough (winget commands, PATH setup, verification,
troubleshooting) rather than the terse version above, see
[INSTALL.md](INSTALL.md).

## Build

```bash
cd go
go build -o mediasearch ./cmd/mediasearch
```

Cross-compile a Windows executable from any platform (no cgo, so this just
works):

```bash
GOOS=windows GOARCH=amd64 go build -o mediasearch.exe ./cmd/mediasearch
```

Other targets follow the same pattern, e.g. `GOOS=darwin GOARCH=arm64` for
Apple Silicon macOS, or `GOOS=linux GOARCH=arm64` for Linux on ARM.

## Usage

Identical to the Python version's CLI (see the [project
README](../README.md#usage) for more examples):

```bash
./mediasearch --input photo.png --text "invoice number"
./mediasearch --input recording.mp4 --template logo.png
./mediasearch --input contract.pdf --text "termination clause"
```

Run `./mediasearch --help` for the full flag reference. Output is the same
JSON shape as the Python version (see [project
README](../README.md#output)), with one additional error-message
difference: this version's errors always name the specific missing tool
(e.g. `required tool 'ffmpeg' was not found on PATH`) rather than a generic
failure.

## How image matching differs from the Python version

The Python version uses OpenCV's `matchTemplate` (normalized
cross-correlation) to locate a candidate, then validates it with pixel-color
similarity — a fix for a real bug where a flat/textureless template scores a
spurious perfect correlation against any unrelated flat-colored region
(the correlation coefficient's normalization is a 0/0 case for zero
variance).

This Go version has no correlation routine to lean on, so it uses pixel-color
similarity (1 − normalized mean absolute difference) as the *only* metric,
throughout. That sidesteps the degenerate case by construction rather than
needing a second validation pass — but without OpenCV's optimized routines,
a brute-force pixel-difference search at full resolution would be far too
slow on anything but a small image. So matching is coarse-to-fine:
localization runs on a copy of the image downscaled to at most
`--match-max-dim` pixels on its longest side (default 180), and the final
confidence is then computed once, at full resolution, for that single
winning candidate.

Practical effect: reported `bbox` coordinates are quantized to the
downscale factor (typically within a few pixels for a moderately sized
image), and matching cost grows roughly with `--match-max-dim` to the 4th
power — raise it for more localization precision on a large image, at a
steep speed cost; lower it for faster, less precise matching.

## Known limitations

Same categories as the Python version (video sampling gaps, per-line-only
text matching, no rotation/perspective invariance in template matching) —
see the [project README](../README.md#known-limitations). Additionally:

- Image decoding supports PNG/JPEG/GIF/BMP/TIFF/WebP (via
  `golang.org/x/image`) — narrower than what a full OpenCV install decodes,
  but covers all common cases.
- PDF line-grouping is a geometry heuristic (clustering words by vertical
  overlap, since `pdftotext -bbox` doesn't report line indices the way
  PyMuPDF's `get_text("words")` does) rather than the PDF's own layout
  structure. Behaves the same for ordinary horizontal text; unusual layouts
  (multi-column, rotated text) may group lines differently than the Python
  version.

## Tests

```bash
cd go
go test ./...
```

Fixtures are generated at test time (an OCR-readable text image via
`golang.org/x/image/font/basicfont`, a synthetic scene + template crop, and
a short video encoded with `ffmpeg`), except the PDF fixture
(`testdata/sample.pdf`), which is committed statically since Go has no
practical way to generate a PDF without a new, heavier dependency.

## Layout

```
cmd/mediasearch/        entry point
internal/
  extract/               loads image/video/pdf into a common sequence of "units"
  textmatch/             text search: PDF text layer (pdftotext -bbox), or tesseract OCR
  imagematch/            native Go template image search
  cli/                   argument parsing, orchestration, JSON output
  exectool/, apperr/     shared "run external tool" / "expected error" helpers
  testfixture/           synthetic test fixtures shared across packages
testdata/sample.pdf      static PDF fixture (see Tests)
```
