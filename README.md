# testing01

## mediasearch

A command-line tool that searches an image, video, or PDF for either a text
string or a template image, and reports the result as JSON — whether it was
found, and where.

- **Text search**: for images and video frames, runs OCR (Tesseract) and
  does a case-insensitive substring search over the recognized text. For
  PDFs, it uses the page's embedded text layer directly when the page has
  one (fast and exact), and only falls back to OCR on pages that don't (e.g.
  a scanned page with no text layer).
- **Image search**: multi-scale template matching (OpenCV), so the target
  image can appear at a different size than the template you supply.
- **Video**: sampled at a configurable interval (default: 1 frame/second),
  not decoded frame-by-frame, so it stays fast on longer videos.

### Install

System dependency (the actual OCR engine; `pytesseract` is just a wrapper
around it):

```bash
apt-get install tesseract-ocr        # Debian/Ubuntu
brew install tesseract               # macOS
```

Python dependencies:

```bash
pip install -r requirements.txt
```

### Usage

```bash
# Is this text anywhere in the image/video/PDF?
python3 -m mediasearch --input photo.png --text "invoice number"

# Is this image (a logo, icon, screenshot fragment, ...) anywhere in it?
python3 -m mediasearch --input recording.mp4 --template logo.png

# Search a PDF, sampling every page (frame-interval is video-only)
python3 -m mediasearch --input contract.pdf --text "termination clause"

# Slower/denser video sampling, custom match threshold, write to a file
python3 -m mediasearch --input recording.mp4 --template logo.png \
    --frame-interval 0.5 --threshold 0.85 --output result.json
```

Run `python3 -m mediasearch --help` for the full flag reference
(`--case-sensitive`, `--lang`, `--dpi`, `--max-units`, `--compact`, etc).

### Output

One JSON object on stdout (or written to `--output`), e.g.:

```json
{
  "input": "recording.mp4",
  "input_type": "video",
  "query_type": "text",
  "query": "invoice number",
  "found": true,
  "match_count": 1,
  "matches": [
    {
      "unit_type": "frame",
      "unit_index": 3,
      "timestamp_sec": 3.0,
      "matched_text": "Invoice Number",
      "source": "ocr",
      "confidence": 0.87,
      "bbox": { "x": 68, "y": 102, "width": 58, "height": 8 }
    }
  ]
}
```

- `unit_type`/`unit_index`: which image/frame/page the match was in
  (`timestamp_sec` is also present for video frames).
- `source` (text search only): `"pdf_text"` (exact, confidence always `1.0`)
  or `"ocr"` (confidence 0–1, from Tesseract).
- `confidence` (template search): `1.0` for a pixel-identical match, based
  on actual pixel-color similarity at the best-scoring location.
- `bbox`: pixel location of the match in that unit's image.

On failure (bad path, unreadable file, unsupported format, empty
`--text`/`--template` value), it prints `{"input": ..., "error": "..."}` and
exits non-zero instead of a `found`/`matches` result, so a caller can always
parse stdout as JSON and branch on whether `"error"` is present.

### Known limitations

- **Video sampling**: text/images present for less than one sampling
  interval can be missed between samples. Lower `--frame-interval` for more
  thorough (slower) coverage.
- **Text matching is per-line**: a query is matched against each OCR/PDF
  line individually, so a phrase that OCR splits across two lines won't be
  found even though it's visually present. Multi-line queries aren't
  supported.
- **Template matching is scale-invariant but not rotation/perspective-invariant**:
  it handles the target appearing larger/smaller than the template, not
  rotated or viewed at an angle. It's also less reliable for templates with
  very little internal detail (e.g. a mostly solid-color logo), since
  there's less genuine pixel structure to confirm a real match against.
- **OCR quality** depends on image resolution, font, and language; pass
  `--lang` for non-English text (any Tesseract language code).

### Tests

```bash
pip install -r requirements-dev.txt
python3 -m pytest tests/
```

### Layout

```
mediasearch/
  extract.py      # loads image/video/pdf into a common sequence of "units"
  text_match.py   # text search: PDF text layer, or OCR fallback
  image_match.py  # template image search
  cli.py          # argument parsing, orchestration, JSON output
```
