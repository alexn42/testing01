import cv2
import numpy as np

from mediasearch import extract, image_match


def _first_unit(path):
    _, units = extract.iter_units(path)
    return next(units)


def test_exact_crop_found_with_high_confidence(scene_and_template):
    unit = _first_unit(scene_and_template["scene"])
    template = cv2.imread(str(scene_and_template["template"]))
    matches = image_match.search_image_unit(unit, template)
    assert len(matches) == 1
    assert matches[0]["confidence"] > 0.95
    bbox = matches[0]["bbox"]
    expected = scene_and_template["template_bbox"]
    assert abs(bbox["x"] - expected["x"]) <= 2
    assert abs(bbox["y"] - expected["y"]) <= 2


def test_textured_template_not_present_is_not_found(scene_and_template):
    unit = _first_unit(scene_and_template["scene"])
    # A black/white checkerboard: real texture (not a degenerate flat
    # color), and a pattern that never appears in the scene (which only has
    # solid-fill shapes on a plain background).
    checkerboard = np.full((60, 60, 3), 255, dtype=np.uint8)
    for r in range(0, 60, 10):
        for c in range(0, 60, 10):
            if (r // 10 + c // 10) % 2 == 0:
                checkerboard[r : r + 10, c : c + 10] = (0, 0, 0)
    assert image_match.search_image_unit(unit, checkerboard) == []


def test_flat_color_template_not_falsely_matched(scene_and_template):
    # Regression test: cv2.TM_CCOEFF_NORMED's normalization divides by the
    # template's variance, so a flat/textureless (solid color) template is a
    # 0/0 case that scored a spurious 1.0 against an unrelated flat region
    # before the color-similarity validation step was added.
    unit = _first_unit(scene_and_template["scene"])
    template = cv2.imread(str(scene_and_template["negative_template"]))
    assert image_match.search_image_unit(unit, template) == []


def test_scaled_template_still_found(scene_and_template):
    template = cv2.imread(str(scene_and_template["template"]))
    shrunk = cv2.resize(template, None, fx=0.75, fy=0.75)
    unit = _first_unit(scene_and_template["scene"])
    matches = image_match.search_image_unit(unit, shrunk)
    assert len(matches) == 1
    assert matches[0]["confidence"] > 0.9


def test_below_threshold_match_is_not_reported(scene_and_template):
    unit = _first_unit(scene_and_template["scene"])
    template = cv2.imread(str(scene_and_template["template"]))
    assert image_match.search_image_unit(unit, template, threshold=1.01) == []
