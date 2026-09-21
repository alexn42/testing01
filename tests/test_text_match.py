import pytest

from mediasearch import extract, text_match
from mediasearch.text_match import _Word, _search_lines


def _first_unit(path):
    _, units = extract.iter_units(path)
    return next(units)


def test_finds_substring_case_insensitive(text_image_path):
    matches = text_match.search_text_unit(_first_unit(text_image_path), "mediasearch")
    assert len(matches) == 1
    assert matches[0]["source"] == "ocr"
    assert matches[0]["matched_text"].lower() == "mediasearch"
    assert 0.0 < matches[0]["confidence"] <= 1.0


def test_absent_text_not_found(text_image_path):
    assert text_match.search_text_unit(_first_unit(text_image_path), "goodbye") == []


def test_case_sensitive_rejects_wrong_case(text_image_path):
    unit = _first_unit(text_image_path)
    assert text_match.search_text_unit(unit, "MEDIASEARCH", case_sensitive=True) == []
    assert text_match.search_text_unit(unit, "Mediasearch", case_sensitive=True) != []


def test_empty_query_is_rejected(text_image_path):
    with pytest.raises(ValueError):
        text_match.search_text_unit(_first_unit(text_image_path), "")


def test_multiple_occurrences_in_same_line_all_reported():
    # Deterministic unit test of the line-search/span logic itself, bypassing
    # OCR so it can't be flaky.
    words = [
        _Word(text="cat", x0=0, y0=0, x1=10, y1=10, conf=0.9),
        _Word(text="dog", x0=12, y0=0, x1=22, y1=10, conf=0.9),
        _Word(text="cat", x0=24, y0=0, x1=34, y1=10, conf=0.9),
    ]
    matches = _search_lines([words], "cat", case_sensitive=False, source="ocr")
    assert len(matches) == 2
    assert all(m["matched_text"] == "cat" for m in matches)
    assert [m["bbox"]["x"] for m in matches] == [0, 24]
