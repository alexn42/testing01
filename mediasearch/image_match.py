"""Search a Unit for a template image using multi-scale template matching.

Matching has two stages per scale: grayscale normalized cross-correlation
(cv2.TM_CCOEFF_NORMED) proposes a candidate location, then that candidate is
scored by actual color-pixel similarity against the source. The second stage
is required, not optional: TM_CCOEFF_NORMED's normalization divides by the
template's variance, so a flat/textureless template (e.g. a solid-color
crop) is a 0/0 case that can score a spurious 1.0 against any unrelated
region. Grounding the reported confidence in real pixel differences catches
that. This handles scale differences between the template and its
appearance in the source by resizing the template across a range of scales.
It does not handle rotation or perspective distortion — see README.
"""

from __future__ import annotations

from typing import List, Optional, Sequence

import cv2
import numpy as np

from mediasearch.extract import Unit

DEFAULT_THRESHOLD = 0.8
DEFAULT_SCALES = tuple(round(s, 2) for s in np.linspace(0.5, 1.5, 11))
MIN_TEMPLATE_SIDE_PX = 4


def search_image_unit(
    unit: Unit,
    template_bgr: np.ndarray,
    threshold: float = DEFAULT_THRESHOLD,
    scales: Optional[Sequence[float]] = None,
) -> List[dict]:
    """Return a single best match (as a one-item list) if it clears `threshold`, else []."""
    if template_bgr is None or template_bgr.size == 0:
        raise ValueError("template image is empty")
    if scales is None:
        scales = DEFAULT_SCALES

    haystack_bgr = unit.image
    haystack_gray = cv2.cvtColor(haystack_bgr, cv2.COLOR_BGR2GRAY)
    template_gray = cv2.cvtColor(template_bgr, cv2.COLOR_BGR2GRAY)
    th0, tw0 = template_gray.shape[:2]
    hh, hw = haystack_gray.shape[:2]

    candidate = None
    for scale in scales:
        tw = max(1, round(tw0 * scale))
        th = max(1, round(th0 * scale))
        if tw > hw or th > hh or tw < MIN_TEMPLATE_SIDE_PX or th < MIN_TEMPLATE_SIDE_PX:
            continue
        interp = cv2.INTER_AREA if scale < 1 else cv2.INTER_LINEAR
        resized = cv2.resize(template_gray, (tw, th), interpolation=interp)
        result = cv2.matchTemplate(haystack_gray, resized, cv2.TM_CCOEFF_NORMED)
        _, max_val, _, max_loc = cv2.minMaxLoc(result)
        if candidate is None or max_val > candidate["correlation"]:
            candidate = {
                "correlation": float(max_val),
                "x": int(max_loc[0]),
                "y": int(max_loc[1]),
                "width": tw,
                "height": th,
                "scale": float(scale),
            }

    if candidate is None:
        return []

    x, y, w, h = candidate["x"], candidate["y"], candidate["width"], candidate["height"]
    interp = cv2.INTER_AREA if candidate["scale"] < 1 else cv2.INTER_LINEAR
    resized_template_bgr = cv2.resize(template_bgr, (w, h), interpolation=interp)
    window_bgr = haystack_bgr[y : y + h, x : x + w]
    mean_abs_diff = float(
        np.mean(np.abs(window_bgr.astype(np.float32) - resized_template_bgr.astype(np.float32)))
    )
    confidence = 1.0 - (mean_abs_diff / 255.0)

    if confidence < threshold:
        return []
    return [
        {
            "confidence": round(confidence, 4),
            "bbox": {"x": x, "y": y, "width": w, "height": h},
            "scale": candidate["scale"],
        }
    ]
