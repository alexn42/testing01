"""Search a Unit for a text query.

Two sources feed the same line-grouping/substring-search logic:
  - "pdf_text": words from a PDF's embedded text layer (exact, confidence 1.0).
  - "ocr":      words from Tesseract OCR on the rendered image (confidence 0..1).
"""

from __future__ import annotations

from dataclasses import dataclass
from typing import Dict, Iterator, List, Optional, Sequence, Tuple

import cv2
import pytesseract
from pytesseract import Output

from mediasearch.extract import Unit, WordBox

DEFAULT_LANG = "eng"


@dataclass
class _Word:
    text: str
    x0: float
    y0: float
    x1: float
    y1: float
    conf: Optional[float]  # 0..1 for OCR words; None for exact digital text


def search_text_unit(
    unit: Unit, query: str, case_sensitive: bool = False, lang: str = DEFAULT_LANG
) -> List[dict]:
    if not query:
        raise ValueError("query must be a non-empty string")

    if unit.text_words:
        lines = _group_pdf_words(unit.text_words)
        source = "pdf_text"
    else:
        lines = _group_ocr_words(unit.image, lang)
        source = "ocr"

    return _search_lines(lines, query, case_sensitive, source)


def _group_pdf_words(words: Sequence[WordBox]) -> List[List[_Word]]:
    lines: Dict[Tuple[int, int], List[_Word]] = {}
    for w in words:
        key = (w.block, w.line)
        lines.setdefault(key, []).append(
            _Word(text=w.text, x0=w.x0, y0=w.y0, x1=w.x1, y1=w.y1, conf=None)
        )
    return list(lines.values())


def _group_ocr_words(image_bgr, lang: str) -> List[List[_Word]]:
    rgb = cv2.cvtColor(image_bgr, cv2.COLOR_BGR2RGB)
    data = pytesseract.image_to_data(rgb, lang=lang, output_type=Output.DICT)
    lines: Dict[Tuple[int, int, int], List[_Word]] = {}
    for i in range(len(data["text"])):
        text = data["text"][i].strip()
        if not text:
            continue
        conf_raw = float(data["conf"][i])
        conf = conf_raw / 100.0 if conf_raw >= 0 else None
        key = (data["block_num"][i], data["par_num"][i], data["line_num"][i])
        x0 = float(data["left"][i])
        y0 = float(data["top"][i])
        lines.setdefault(key, []).append(
            _Word(
                text=text,
                x0=x0,
                y0=y0,
                x1=x0 + float(data["width"][i]),
                y1=y0 + float(data["height"][i]),
                conf=conf,
            )
        )
    return list(lines.values())


def _search_lines(
    lines: List[List[_Word]], query: str, case_sensitive: bool, source: str
) -> List[dict]:
    needle = query if case_sensitive else query.lower()
    matches = []
    for words in lines:
        line_text, offsets = _join_words(words)
        haystack = line_text if case_sensitive else line_text.lower()
        for m_start, m_end in _find_all(haystack, needle):
            hit_words = [
                w
                for w, (o_start, o_end) in zip(words, offsets)
                if o_start < m_end and o_end > m_start
            ]
            if not hit_words:
                continue
            confs = [w.conf for w in hit_words if w.conf is not None]
            matches.append(
                {
                    "matched_text": " ".join(w.text for w in hit_words),
                    "source": source,
                    "confidence": round(sum(confs) / len(confs), 4) if confs else 1.0,
                    "bbox": _union_bbox(hit_words),
                }
            )
    return matches


def _join_words(words: Sequence[_Word]) -> Tuple[str, List[Tuple[int, int]]]:
    parts = []
    offsets = []
    cursor = 0
    for w in words:
        start = cursor
        end = start + len(w.text)
        offsets.append((start, end))
        parts.append(w.text)
        cursor = end + 1  # +1 for the joining space
    return " ".join(parts), offsets


def _find_all(haystack: str, needle: str) -> Iterator[Tuple[int, int]]:
    if not needle:
        return
    start = 0
    while True:
        idx = haystack.find(needle, start)
        if idx == -1:
            return
        yield (idx, idx + len(needle))
        start = idx + 1  # allow overlapping matches


def _union_bbox(words: Sequence[_Word]) -> dict:
    x0 = min(w.x0 for w in words)
    y0 = min(w.y0 for w in words)
    x1 = max(w.x1 for w in words)
    y1 = max(w.y1 for w in words)
    return {
        "x": round(x0),
        "y": round(y0),
        "width": round(x1 - x0),
        "height": round(y1 - y0),
    }
