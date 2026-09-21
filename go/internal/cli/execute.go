package cli

import (
	"errors"
	"fmt"

	"mediasearch/internal/extract"
	"mediasearch/internal/imagematch"
	"mediasearch/internal/textmatch"
)

type BBox struct {
	X      int `json:"x"`
	Y      int `json:"y"`
	Width  int `json:"width"`
	Height int `json:"height"`
}

type Match struct {
	UnitType     string   `json:"unit_type"`
	UnitIndex    int      `json:"unit_index"`
	TimestampSec *float64 `json:"timestamp_sec,omitempty"`
	MatchedText  string   `json:"matched_text,omitempty"`
	Source       string   `json:"source,omitempty"`
	Confidence   float64  `json:"confidence"`
	BBox         BBox     `json:"bbox"`
	Scale        *float64 `json:"scale,omitempty"`
}

type successResult struct {
	Input      string  `json:"input"`
	InputType  string  `json:"input_type"`
	QueryType  string  `json:"query_type"`
	Query      string  `json:"query"`
	Found      bool    `json:"found"`
	MatchCount int     `json:"match_count"`
	Matches    []Match `json:"matches"`
}

func execute(cfg *Config) (*successResult, error) {
	queryType, queryValue, tmpl, err := resolveQuery(cfg)
	if err != nil {
		return nil, err
	}

	sess, err := extract.Load(cfg.Input, extract.Options{
		FrameInterval: cfg.FrameInterval,
		MaxUnits:      cfg.MaxUnits,
		DPI:           cfg.DPI,
	})
	if err != nil {
		return nil, err
	}
	defer sess.Close()

	matches := []Match{} // never nil: must serialize as [] rather than null
	for _, unit := range sess.Units {
		unitMatches, err := searchUnit(unit, queryType, queryValue, tmpl, cfg)
		if err != nil {
			return nil, err
		}
		matches = append(matches, unitMatches...)
	}

	return &successResult{
		Input: cfg.Input, InputType: sess.InputType, QueryType: queryType, Query: queryValue,
		Found: len(matches) > 0, MatchCount: len(matches), Matches: matches,
	}, nil
}

func resolveQuery(cfg *Config) (queryType, queryValue string, tmpl *imagematch.Template, err error) {
	if cfg.Text != nil {
		if *cfg.Text == "" {
			return "", "", nil, errors.New("--text must not be empty")
		}
		return "text", *cfg.Text, nil, nil
	}

	if *cfg.Template == "" {
		return "", "", nil, errors.New("--template must not be empty")
	}
	tmpl, err = imagematch.LoadTemplate(*cfg.Template)
	if err != nil {
		return "", "", nil, fmt.Errorf("could not decode template image: %s: %w", *cfg.Template, err)
	}
	return "template_image", *cfg.Template, tmpl, nil
}

func searchUnit(
	unit extract.Unit, queryType, queryValue string, tmpl *imagematch.Template, cfg *Config,
) ([]Match, error) {
	if queryType == "text" {
		found, err := textmatch.SearchTextUnit(unit, queryValue, cfg.CaseSensitive, cfg.Lang)
		if err != nil {
			return nil, err
		}
		matches := make([]Match, len(found))
		for i, m := range found {
			matches[i] = Match{
				UnitType: unit.UnitType, UnitIndex: unit.Index, TimestampSec: unit.TimestampSec,
				MatchedText: m.MatchedText, Source: m.Source, Confidence: m.Confidence,
				BBox: BBox{X: m.BBox.X, Y: m.BBox.Y, Width: m.BBox.Width, Height: m.BBox.Height},
			}
		}
		return matches, nil
	}

	found, err := imagematch.SearchImageUnit(unit, tmpl, cfg.Threshold, nil, cfg.MatchMaxDim)
	if err != nil {
		return nil, err
	}
	matches := make([]Match, len(found))
	for i, m := range found {
		scale := m.Scale
		matches[i] = Match{
			UnitType: unit.UnitType, UnitIndex: unit.Index, TimestampSec: unit.TimestampSec,
			Confidence: m.Confidence,
			BBox:       BBox{X: m.BBox.X, Y: m.BBox.Y, Width: m.BBox.Width, Height: m.BBox.Height},
			Scale:      &scale,
		}
	}
	return matches, nil
}
