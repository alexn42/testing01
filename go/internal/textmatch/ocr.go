package textmatch

import (
	"bufio"
	"bytes"
	"strconv"
	"strings"

	"mediasearch/internal/exectool"
)

const (
	tesseractBin = "tesseract"
	// tesseractTSVWordLevel is the "level" column value for a word-level row
	// in tesseract's TSV output (1=page, 2=block, 3=paragraph, 4=line, 5=word).
	tesseractTSVWordLevel = "5"
)

// groupOCRWords runs `tesseract <path> stdout -l <lang> tsv` and groups its
// word-level rows into lines by (block_num, par_num, line_num).
func groupOCRWords(path, lang string) ([][]word, error) {
	if lang == "" {
		lang = DefaultLang
	}
	out, err := exectool.Run(tesseractBin, path, "stdout", "-l", lang, "tsv")
	if err != nil {
		return nil, err
	}
	return parseTesseractTSV(out), nil
}

func parseTesseractTSV(out []byte) [][]word {
	type lineKey struct{ block, par, line int }
	var order []lineKey
	byLine := map[lineKey][]word{}

	scanner := bufio.NewScanner(bytes.NewReader(out))
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	first := true
	for scanner.Scan() {
		if first {
			first = false
			continue // header row
		}
		fields := strings.Split(scanner.Text(), "\t")
		if len(fields) < 12 || fields[0] != tesseractTSVWordLevel {
			continue
		}
		text := strings.TrimSpace(fields[11])
		if text == "" {
			continue
		}
		block, _ := strconv.Atoi(fields[2])
		par, _ := strconv.Atoi(fields[3])
		ln, _ := strconv.Atoi(fields[4])
		left, _ := strconv.ParseFloat(fields[6], 64)
		top, _ := strconv.ParseFloat(fields[7], 64)
		width, _ := strconv.ParseFloat(fields[8], 64)
		height, _ := strconv.ParseFloat(fields[9], 64)

		var conf *float64
		if confRaw, err := strconv.ParseFloat(fields[10], 64); err == nil && confRaw >= 0 {
			c := confRaw / 100.0
			conf = &c
		}

		key := lineKey{block, par, ln}
		if _, ok := byLine[key]; !ok {
			order = append(order, key)
		}
		byLine[key] = append(byLine[key], word{
			text: text,
			x0:   left, y0: top,
			x1: left + width, y1: top + height,
			conf: conf,
		})
	}

	lines := make([][]word, 0, len(order))
	for _, k := range order {
		lines = append(lines, byLine[k])
	}
	return lines
}
