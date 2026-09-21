// Package textmatch searches a Unit for a text query.
//
// Two sources feed the same line-grouping/substring-search logic:
//   - "pdf_text": words from a PDF's embedded text layer (exact, confidence 1.0).
//   - "ocr":      words from Tesseract OCR on the unit's image file (confidence 0..1).
package textmatch

import (
	"errors"
	"strings"

	"mediasearch/internal/extract"
)

const DefaultLang = "eng"

type BBox struct {
	X, Y, Width, Height int
}

type Match struct {
	MatchedText string
	Source      string // "pdf_text" | "ocr"
	Confidence  float64
	BBox        BBox
}

type word struct {
	text           string
	x0, y0, x1, y1 float64
	conf           *float64 // nil for exact digital text
}

func SearchTextUnit(unit extract.Unit, query string, caseSensitive bool, lang string) ([]Match, error) {
	if query == "" {
		return nil, errors.New("query must be a non-empty string")
	}

	var lines [][]word
	var source string
	if unit.TextWords != nil {
		lines = groupPDFWords(unit.TextWords)
		source = "pdf_text"
	} else {
		var err error
		lines, err = groupOCRWords(unit.Path, lang)
		if err != nil {
			return nil, err
		}
		source = "ocr"
	}

	return searchLines(lines, query, caseSensitive, source), nil
}

// groupPDFWords relies on extract's WordBox.Line already being sorted (it
// is: extract.groupIntoLines sorts by Line then X0 before returning).
func groupPDFWords(words []extract.WordBox) [][]word {
	var lines [][]word
	var current []word
	currentLine := -1
	for _, w := range words {
		if w.Line != currentLine {
			if len(current) > 0 {
				lines = append(lines, current)
			}
			current = nil
			currentLine = w.Line
		}
		current = append(current, word{text: w.Text, x0: w.X0, y0: w.Y0, x1: w.X1, y1: w.Y1})
	}
	if len(current) > 0 {
		lines = append(lines, current)
	}
	return lines
}

func searchLines(lines [][]word, query string, caseSensitive bool, source string) []Match {
	needle := query
	if !caseSensitive {
		needle = strings.ToLower(query)
	}

	var matches []Match
	for _, words := range lines {
		lineText, offsets := joinWords(words)
		haystack := lineText
		if !caseSensitive {
			haystack = strings.ToLower(lineText)
		}
		for _, span := range findAll(haystack, needle) {
			var hitWords []word
			for i, off := range offsets {
				if off.start < span.end && off.end > span.start {
					hitWords = append(hitWords, words[i])
				}
			}
			if len(hitWords) == 0 {
				continue
			}
			matches = append(matches, Match{
				MatchedText: joinText(hitWords),
				Source:      source,
				Confidence:  averageConfidence(hitWords),
				BBox:        unionBBox(hitWords),
			})
		}
	}
	return matches
}

type charSpan struct{ start, end int }

// joinWords concatenates word texts with single spaces (mirroring how
// tesseract/pdftotext already tokenize on whitespace) and returns each
// word's [start,end) character offset in that joined string, so a match
// span can be mapped back to the specific words it covers.
func joinWords(words []word) (string, []charSpan) {
	var b strings.Builder
	offsets := make([]charSpan, len(words))
	cursor := 0
	for i, w := range words {
		start := cursor
		end := start + len(w.text)
		offsets[i] = charSpan{start, end}
		if i > 0 {
			b.WriteByte(' ')
		}
		b.WriteString(w.text)
		cursor = end + 1 // +1 for the joining space
	}
	return b.String(), offsets
}

// findAll returns every (possibly overlapping) occurrence of needle in
// haystack as [start,end) spans.
func findAll(haystack, needle string) []charSpan {
	if needle == "" {
		return nil
	}
	var spans []charSpan
	start := 0
	for {
		idx := strings.Index(haystack[start:], needle)
		if idx == -1 {
			return spans
		}
		absIdx := start + idx
		spans = append(spans, charSpan{absIdx, absIdx + len(needle)})
		start = absIdx + 1 // allow overlapping matches
	}
}

func joinText(words []word) string {
	parts := make([]string, len(words))
	for i, w := range words {
		parts[i] = w.text
	}
	return strings.Join(parts, " ")
}

func averageConfidence(words []word) float64 {
	var sum float64
	var n int
	for _, w := range words {
		if w.conf != nil {
			sum += *w.conf
			n++
		}
	}
	if n == 0 {
		return 1.0 // exact digital text: not a probabilistic OCR guess
	}
	return round4(sum / float64(n))
}

func unionBBox(words []word) BBox {
	x0, y0 := words[0].x0, words[0].y0
	x1, y1 := words[0].x1, words[0].y1
	for _, w := range words[1:] {
		x0 = min(x0, w.x0)
		y0 = min(y0, w.y0)
		x1 = max(x1, w.x1)
		y1 = max(y1, w.y1)
	}
	return BBox{
		X: int(x0 + 0.5), Y: int(y0 + 0.5),
		Width: int(x1 - x0 + 0.5), Height: int(y1 - y0 + 0.5),
	}
}

func round4(v float64) float64 {
	return float64(int(v*10000+0.5)) / 10000
}
