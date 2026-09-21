from mediasearch import extract, text_match


def _units(path):
    _, units = extract.iter_units(path)
    return list(units)


def test_pdf_yields_one_unit_per_page(sample_pdf_path):
    units = _units(sample_pdf_path)
    assert [u.unit_type for u in units] == ["page", "page", "page"]
    assert [u.index for u in units] == [0, 1, 2]


def test_digital_text_layer_used_for_normal_pages(sample_pdf_path):
    page0 = _units(sample_pdf_path)[0]
    assert page0.text_words  # fixture page has a real embedded text layer
    matches = text_match.search_text_unit(page0, "quick brown fox")
    assert len(matches) == 1
    assert matches[0]["source"] == "pdf_text"
    assert matches[0]["confidence"] == 1.0


def test_ocr_fallback_for_page_with_no_text_layer(sample_pdf_path):
    scanned_page = _units(sample_pdf_path)[2]
    assert scanned_page.text_words == []  # image-only page: no embedded text
    matches = text_match.search_text_unit(scanned_page, "mediasearch")
    assert len(matches) == 1
    assert matches[0]["source"] == "ocr"


def test_match_is_not_reported_on_wrong_page(sample_pdf_path):
    units = _units(sample_pdf_path)
    assert text_match.search_text_unit(units[1], "quick brown fox") == []
    assert text_match.search_text_unit(units[0], "nothing interesting") == []


def test_text_absent_from_every_page(sample_pdf_path):
    for unit in _units(sample_pdf_path):
        assert text_match.search_text_unit(unit, "nonexistent phrase xyz") == []
