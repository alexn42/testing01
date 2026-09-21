// Package cli is the command-line interface: search an image/video/PDF for
// text or a template image, and print a JSON report.
package cli

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"mediasearch/internal/extract"
	"mediasearch/internal/imagematch"
	"mediasearch/internal/textmatch"
)

const usage = `mediasearch: search an image, video, or PDF for text or a template image,
and print a JSON report of whether (and where) it was found.

Usage:
  mediasearch --input FILE (--text QUERY | --template IMAGE) [options]

Required:
  --input, -i FILE        Path to the image, video, or PDF to search.
  --text, -t QUERY        Text to search for (case-insensitive substring by default).
  --template IMAGE        Path to an image file to search for within the input.
                           Exactly one of --text or --template is required.

Options:
  --threshold FLOAT        Minimum match confidence (0-1) for --template matches. Default 0.8.
  --case-sensitive          Require exact case for --text matches.
  --frame-interval SECONDS  Seconds between sampled video frames. Default 1.
  --max-units N             Cap on frames/pages processed (0 = no cap). Default 0.
  --lang CODE               Tesseract language code used for OCR. Default eng.
  --dpi N                   DPI used to render PDF pages before matching. Default 200.
  --match-max-dim N         Resolution cap for --template localization; raising it improves
                             precision on large images at a steep speed cost (roughly N^4).
                             Default 180.
  --output, -o FILE         Write JSON to this file instead of stdout.
  --compact                 Emit compact JSON instead of indented JSON.
`

// Config is the fully validated, parsed command line.
type Config struct {
	Input         string
	Text          *string // nil = --text not given
	Template      *string // nil = --template not given
	Threshold     float64
	CaseSensitive bool
	FrameInterval float64
	MaxUnits      int
	Lang          string
	DPI           int
	MatchMaxDim   int
	Output        string
	Compact       bool
}

// Main is the process entry point's logic, returning the process exit code.
func Main(args []string) int {
	cfg, err := ParseArgs(args)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			fmt.Print(usage)
			return 0
		}
		fmt.Fprintln(os.Stderr, "mediasearch:", err)
		fmt.Fprint(os.Stderr, usage)
		return 2
	}
	return run(cfg)
}

func ParseArgs(args []string) (*Config, error) {
	fs := flag.NewFlagSet("mediasearch", flag.ContinueOnError)
	fs.SetOutput(io.Discard) // Main prints its own error + usage, uniformly

	var input, text, template, lang, output string
	var threshold float64
	var caseSensitive, compact bool
	var frameInterval float64
	var maxUnits, dpi, matchMaxDim int

	fs.StringVar(&input, "input", "", "")
	fs.StringVar(&input, "i", "", "")
	fs.StringVar(&text, "text", "", "")
	fs.StringVar(&text, "t", "", "")
	fs.StringVar(&template, "template", "", "")
	fs.Float64Var(&threshold, "threshold", imagematch.DefaultThreshold, "")
	fs.BoolVar(&caseSensitive, "case-sensitive", false, "")
	fs.Float64Var(&frameInterval, "frame-interval", extract.DefaultFrameIntervalSec, "")
	fs.IntVar(&maxUnits, "max-units", 0, "")
	fs.StringVar(&lang, "lang", textmatch.DefaultLang, "")
	fs.IntVar(&dpi, "dpi", extract.DefaultPDFDPI, "")
	fs.IntVar(&matchMaxDim, "match-max-dim", imagematch.DefaultMatchMaxDim, "")
	fs.StringVar(&output, "output", "", "")
	fs.StringVar(&output, "o", "", "")
	fs.BoolVar(&compact, "compact", false, "")

	if err := fs.Parse(args); err != nil {
		return nil, err
	}

	// flag sets zero-value defaults whether or not a flag was given, so an
	// explicitly empty --text ("") is indistinguishable from an omitted one
	// by value alone -- Visit only reports flags actually present on the
	// command line, which is what "given vs. not given" needs.
	provided := map[string]bool{}
	fs.Visit(func(f *flag.Flag) { provided[f.Name] = true })

	if input == "" {
		return nil, errors.New("--input is required")
	}

	textGiven := provided["text"] || provided["t"]
	templateGiven := provided["template"]
	switch {
	case textGiven && templateGiven:
		return nil, errors.New("--text and --template are mutually exclusive")
	case !textGiven && !templateGiven:
		return nil, errors.New("exactly one of --text or --template is required")
	}

	cfg := &Config{
		Input: input, Threshold: threshold, CaseSensitive: caseSensitive,
		FrameInterval: frameInterval, MaxUnits: maxUnits, Lang: lang, DPI: dpi,
		MatchMaxDim: matchMaxDim, Output: output, Compact: compact,
	}
	if textGiven {
		cfg.Text = &text
	} else {
		cfg.Template = &template
	}
	return cfg, nil
}

func run(cfg *Config) int {
	result, err := execute(cfg)
	if err != nil {
		if emitErr := emit(errorResult{Input: cfg.Input, Error: err.Error()}, cfg.Output, cfg.Compact); emitErr != nil {
			fmt.Fprintln(os.Stderr, "mediasearch:", emitErr)
		}
		return 1
	}
	if err := emit(result, cfg.Output, cfg.Compact); err != nil {
		fmt.Fprintln(os.Stderr, "mediasearch:", err)
		return 1
	}
	return 0
}

type errorResult struct {
	Input string `json:"input"`
	Error string `json:"error"`
}

func emit(v interface{}, outputPath string, compact bool) error {
	var data []byte
	var err error
	if compact {
		data, err = json.Marshal(v)
	} else {
		data, err = json.MarshalIndent(v, "", "  ")
	}
	if err != nil {
		return err
	}
	if outputPath != "" {
		return os.WriteFile(outputPath, append(data, '\n'), 0o644)
	}
	fmt.Println(string(data))
	return nil
}
