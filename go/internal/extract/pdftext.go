package extract

import (
	"encoding/xml"
	"math"
	"os"
	"sort"

	"mediasearch/internal/apperr"
	"mediasearch/internal/exectool"
)

// pdfTextWords returns, per 0-indexed page, the words from the PDF's
// embedded text layer (nil for a page with none, e.g. a scanned page),
// scaled from PDF points to pixel coordinates at the given render DPI and
// grouped into lines by vertical position.
func pdfTextWords(path string, dpi int, pageCount int) ([][]WordBox, error) {
	tmp, err := os.CreateTemp("", "mediasearch-bbox-*.html")
	if err != nil {
		return nil, err
	}
	tmpPath := tmp.Name()
	tmp.Close()
	defer os.Remove(tmpPath)

	if _, err := exectool.Run("pdftotext", "-bbox", path, tmpPath); err != nil {
		return nil, err
	}

	data, err := os.ReadFile(tmpPath)
	if err != nil {
		return nil, err
	}
	var doc bboxHTML
	if err := xml.Unmarshal(data, &doc); err != nil {
		return nil, apperr.New("could not parse PDF text layer: %s", err)
	}

	zoom := float64(dpi) / 72.0
	result := make([][]WordBox, pageCount)
	for i, page := range doc.Body.Doc.Pages {
		if i >= pageCount {
			break
		}
		if len(page.Words) == 0 {
			result[i] = nil
			continue
		}
		result[i] = groupIntoLines(page.Words, zoom)
	}
	return result, nil
}

type bboxHTML struct {
	XMLName xml.Name `xml:"html"`
	Body    bboxBody `xml:"body"`
}
type bboxBody struct {
	Doc bboxDoc `xml:"doc"`
}
type bboxDoc struct {
	Pages []bboxPage `xml:"page"`
}
type bboxPage struct {
	Words []bboxWord `xml:"word"`
}
type bboxWord struct {
	XMin float64 `xml:"xMin,attr"`
	YMin float64 `xml:"yMin,attr"`
	XMax float64 `xml:"xMax,attr"`
	YMax float64 `xml:"yMax,attr"`
	Text string  `xml:",chardata"`
}

// groupIntoLines clusters words into visual text lines by vertical overlap:
// poppler's -bbox output gives per-word boxes but no line index of its own
// (unlike PyMuPDF's get_text("words"), which does). Words are sorted
// top-to-bottom then left-to-right, and a word joins the current line if its
// vertical center falls within that line's accumulated y-range (padded by
// half the word's own height, to tolerate normal baseline/ascender jitter).
func groupIntoLines(words []bboxWord, zoom float64) []WordBox {
	sorted := make([]bboxWord, len(words))
	copy(sorted, words)
	sort.SliceStable(sorted, func(i, j int) bool {
		if math.Abs(sorted[i].YMin-sorted[j].YMin) > 0.01 {
			return sorted[i].YMin < sorted[j].YMin
		}
		return sorted[i].XMin < sorted[j].XMin
	})

	result := make([]WordBox, 0, len(sorted))
	line := -1
	var lineYMin, lineYMax float64

	for _, w := range sorted {
		height := w.YMax - w.YMin
		tolerance := height / 2
		center := (w.YMin + w.YMax) / 2
		if line == -1 || center < lineYMin-tolerance || center > lineYMax+tolerance {
			line++
			lineYMin, lineYMax = w.YMin, w.YMax
		} else {
			if w.YMin < lineYMin {
				lineYMin = w.YMin
			}
			if w.YMax > lineYMax {
				lineYMax = w.YMax
			}
		}
		result = append(result, WordBox{
			Text: w.Text,
			X0:   w.XMin * zoom, Y0: w.YMin * zoom,
			X1: w.XMax * zoom, Y1: w.YMax * zoom,
			Line: line,
		})
	}

	sort.SliceStable(result, func(i, j int) bool {
		if result[i].Line != result[j].Line {
			return result[i].Line < result[j].Line
		}
		return result[i].X0 < result[j].X0
	})
	return result
}
