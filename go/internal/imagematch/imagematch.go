// Package imagematch searches a Unit for a template image using multi-scale
// template matching implemented natively (no OpenCV/cgo dependency).
//
// Matching uses actual pixel-color similarity (1 - normalized mean absolute
// difference) throughout, both for locating a candidate position and for
// the final reported confidence -- unlike the Python version's use of
// OpenCV's normalized cross-correlation, whose normalization divides by the
// template's variance and can score a spurious perfect match for a
// flat/textureless template against any unrelated region (a 0/0 case). A
// pixel-difference metric doesn't have that degeneracy: a flat template
// only scores well against a region that is actually a similar color.
//
// To keep runtime bounded regardless of input image size (there is no
// OpenCV-grade optimized correlation routine to lean on here), the search
// is coarse-to-fine: localization runs on a downscaled copy of the haystack
// capped at matchMaxDim on its longest side, then the final confidence is
// computed once, at full resolution, for the single winning candidate.
package imagematch

import (
	"errors"

	"mediasearch/internal/extract"
)

const (
	DefaultThreshold  = 0.8
	minTemplateSidePx = 4
	// DefaultMatchMaxDim bounds the coarse-localization resolution so
	// worst-case cost stays bounded; it does not limit the resolution used
	// for the final confidence check, which always runs at full resolution.
	// Cost scales roughly with this value to the 4th power (both the number
	// of candidate positions and the per-position cost scale with its
	// square), so raising it noticeably slows matching -- e.g. doubling it
	// is roughly a 16x slowdown.
	DefaultMatchMaxDim = 180
)

// DefaultScales mirrors the Python version's np.linspace(0.5, 1.5, 11).
var DefaultScales = []float64{0.5, 0.6, 0.7, 0.8, 0.9, 1.0, 1.1, 1.2, 1.3, 1.4, 1.5}

type BBox struct{ X, Y, Width, Height int }

type Match struct {
	Confidence float64
	BBox       BBox
	Scale      float64
}

// Template is a template image loaded once and reused across many units.
type Template struct {
	color *rgbImage
	gray  *grayImage
}

func LoadTemplate(path string) (*Template, error) {
	img, err := decodeRGB(path)
	if err != nil {
		return nil, err
	}
	if img.w == 0 || img.h == 0 {
		return nil, errors.New("template image is empty")
	}
	return &Template{color: img, gray: img.gray()}, nil
}

// SearchImageUnit returns a single best match (as a one-item slice) if it
// clears threshold, else an empty slice. scales defaults to DefaultScales;
// matchMaxDim <= 0 uses DefaultMatchMaxDim.
func SearchImageUnit(unit extract.Unit, tmpl *Template, threshold float64, scales []float64, matchMaxDim int) ([]Match, error) {
	if scales == nil {
		scales = DefaultScales
	}
	if matchMaxDim <= 0 {
		matchMaxDim = DefaultMatchMaxDim
	}

	haystackColor, err := decodeRGB(unit.Path)
	if err != nil {
		return nil, err
	}
	haystackGray := haystackColor.gray()

	factor := 1
	if longSide := max(haystackGray.w, haystackGray.h); longSide > matchMaxDim {
		factor = (longSide + matchMaxDim - 1) / matchMaxDim
	}
	coarseHaystack := haystackGray
	if factor > 1 {
		coarseHaystack = haystackGray.resizeNearest(
			(haystackGray.w+factor-1)/factor,
			(haystackGray.h+factor-1)/factor,
		)
	}

	type candidate struct {
		x, y, w, h int
		scale      float64
		score      float64
	}
	var best *candidate

	for _, scale := range scales {
		tw := roundInt(float64(tmpl.gray.w) * scale)
		th := roundInt(float64(tmpl.gray.h) * scale)
		if tw < minTemplateSidePx || th < minTemplateSidePx || tw > haystackGray.w || th > haystackGray.h {
			continue
		}

		ctw, cth := max(1, tw/factor), max(1, th/factor)
		if ctw > coarseHaystack.w || cth > coarseHaystack.h {
			continue
		}
		coarseTemplate := tmpl.gray.resizeNearest(ctw, cth)

		cx, cy, score := bestGrayMatch(coarseHaystack, coarseTemplate)
		if best == nil || score > best.score {
			best = &candidate{x: cx * factor, y: cy * factor, w: tw, h: th, scale: scale, score: score}
		}
	}

	if best == nil {
		return nil, nil
	}

	x, y, w, h := clampBox(best.x, best.y, best.w, best.h, haystackColor.w, haystackColor.h)
	resizedTemplateColor := tmpl.color.resizeNearest(w, h)
	windowColor := haystackColor.crop(x, y, w, h)
	confidence := colorSimilarity(windowColor, resizedTemplateColor)

	if confidence < threshold {
		return nil, nil
	}
	return []Match{{
		Confidence: round4(confidence),
		BBox:       BBox{X: x, Y: y, Width: w, Height: h},
		Scale:      best.scale,
	}}, nil
}

// bestGrayMatch brute-force scans every valid position for the best (lowest
// mean-absolute-difference) match of template within haystack.
func bestGrayMatch(haystack, template *grayImage) (x, y int, similarity float64) {
	maxX := haystack.w - template.w
	maxY := haystack.h - template.h
	if maxX < 0 || maxY < 0 {
		return 0, 0, 0
	}
	n := template.w * template.h
	bestSim := -1.0
	var bestX, bestY int
	for py := 0; py <= maxY; py++ {
		for px := 0; px <= maxX; px++ {
			var sum int
			for ty := 0; ty < template.h; ty++ {
				hRow := (py+ty)*haystack.w + px
				tRow := ty * template.w
				for tx := 0; tx < template.w; tx++ {
					d := int(haystack.pix[hRow+tx]) - int(template.pix[tRow+tx])
					if d < 0 {
						d = -d
					}
					sum += d
				}
			}
			sim := 1.0 - float64(sum)/float64(n)/255.0
			if sim > bestSim {
				bestSim, bestX, bestY = sim, px, py
			}
		}
	}
	return bestX, bestY, bestSim
}

func clampBox(x, y, w, h, maxW, maxH int) (int, int, int, int) {
	if x < 0 {
		x = 0
	}
	if y < 0 {
		y = 0
	}
	if x+w > maxW {
		w = maxW - x
	}
	if y+h > maxH {
		h = maxH - y
	}
	return x, y, max(1, w), max(1, h)
}

func roundInt(v float64) int { return int(v + 0.5) }

func round4(v float64) float64 { return float64(int(v*10000+0.5)) / 10000 }
