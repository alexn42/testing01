"""Load images, videos, and PDFs into a common sequence of searchable units."""

from __future__ import annotations

from dataclasses import dataclass
from pathlib import Path
from typing import Iterator, List, Optional, Tuple

import cv2
import numpy as np
import pymupdf

IMAGE_EXTENSIONS = {".png", ".jpg", ".jpeg", ".bmp", ".tiff", ".tif", ".webp", ".gif"}
VIDEO_EXTENSIONS = {".mp4", ".avi", ".mov", ".mkv", ".webm", ".m4v", ".mpg", ".mpeg"}
PDF_EXTENSIONS = {".pdf"}

DEFAULT_FRAME_INTERVAL_SEC = 1.0
DEFAULT_PDF_DPI = 200
FALLBACK_VIDEO_FPS = 25.0


class MediaError(Exception):
    """Raised for expected failures reading or decoding an input file."""


@dataclass
class WordBox:
    """A word from a PDF's embedded text layer, in rendered-pixel coordinates."""

    text: str
    x0: float
    y0: float
    x1: float
    y1: float
    block: int
    line: int


@dataclass
class Unit:
    """One searchable image extracted from the input (a photo, a video frame, a PDF page)."""

    unit_type: str  # "image" | "frame" | "page"
    index: int
    image: np.ndarray  # BGR, shape (H, W, 3), uint8 — OpenCV convention
    timestamp_sec: Optional[float] = None
    # Populated only for PDF pages that have a digital text layer; None means
    # "no text layer available" (plain image/video, or a scanned PDF page),
    # in which case callers should fall back to OCR.
    text_words: Optional[List[WordBox]] = None


def detect_input_type(path: Path) -> str:
    ext = path.suffix.lower()
    if ext in IMAGE_EXTENSIONS:
        return "image"
    if ext in VIDEO_EXTENSIONS:
        return "video"
    if ext in PDF_EXTENSIONS:
        return "pdf"
    supported = sorted(IMAGE_EXTENSIONS | VIDEO_EXTENSIONS | PDF_EXTENSIONS)
    raise MediaError(
        f"Unsupported file extension '{ext}' for '{path.name}'. "
        f"Supported extensions: {', '.join(supported)}"
    )


def iter_units(
    path: Path,
    frame_interval: float = DEFAULT_FRAME_INTERVAL_SEC,
    max_units: Optional[int] = None,
    dpi: int = DEFAULT_PDF_DPI,
) -> Tuple[str, Iterator[Unit]]:
    """Return (input_type, iterator_of_units) for the given file.

    Validates the path and extension eagerly (before any iteration), so a bad
    path or unsupported format fails immediately rather than on first `next()`.
    """
    path = Path(path)
    if not path.is_file():
        raise MediaError(f"Input file not found: {path}")
    input_type = detect_input_type(path)
    return input_type, _dispatch(input_type, path, frame_interval, max_units, dpi)


def _dispatch(
    input_type: str,
    path: Path,
    frame_interval: float,
    max_units: Optional[int],
    dpi: int,
) -> Iterator[Unit]:
    if input_type == "image":
        yield from _iter_image_units(path)
    elif input_type == "video":
        yield from _iter_video_units(path, frame_interval, max_units)
    elif input_type == "pdf":
        yield from _iter_pdf_units(path, max_units, dpi)


def _iter_image_units(path: Path) -> Iterator[Unit]:
    image = cv2.imread(str(path), cv2.IMREAD_COLOR)
    if image is None:
        raise MediaError(f"Could not decode image file: {path}")
    yield Unit(unit_type="image", index=0, image=image)


def _iter_video_units(
    path: Path, frame_interval: float, max_units: Optional[int]
) -> Iterator[Unit]:
    cap = cv2.VideoCapture(str(path))
    if not cap.isOpened():
        raise MediaError(f"Could not open video file: {path}")
    try:
        fps = cap.get(cv2.CAP_PROP_FPS)
        if not fps or fps <= 0:
            fps = FALLBACK_VIDEO_FPS
        step = max(1, round(fps * frame_interval))

        frame_idx = 0
        unit_idx = 0
        while True:
            # grab() decodes nothing; only retrieve() on sampled frames does.
            if not cap.grab():
                break
            if frame_idx % step == 0:
                ok, frame = cap.retrieve()
                if not ok:
                    break
                yield Unit(
                    unit_type="frame",
                    index=unit_idx,
                    image=frame,
                    timestamp_sec=frame_idx / fps,
                )
                unit_idx += 1
                if max_units is not None and unit_idx >= max_units:
                    break
            frame_idx += 1
    finally:
        cap.release()


def _iter_pdf_units(
    path: Path, max_units: Optional[int], dpi: int
) -> Iterator[Unit]:
    try:
        doc = pymupdf.open(str(path))
    except Exception as exc:  # pymupdf raises its own exception types
        raise MediaError(f"Could not open PDF file: {path} ({exc})") from exc
    try:
        zoom = dpi / 72.0  # PDF points are 1/72 inch; scale to the render DPI
        matrix = pymupdf.Matrix(zoom, zoom)
        page_count = len(doc)
        for page_index in range(page_count):
            if max_units is not None and page_index >= max_units:
                break
            page = doc[page_index]

            words_raw = page.get_text("words")
            words = [
                WordBox(
                    text=w[4],
                    x0=w[0] * zoom,
                    y0=w[1] * zoom,
                    x1=w[2] * zoom,
                    y1=w[3] * zoom,
                    block=w[5],
                    line=w[6],
                )
                for w in words_raw
            ]

            pix = page.get_pixmap(matrix=matrix, colorspace=pymupdf.csRGB, alpha=False)
            image_rgb = np.frombuffer(pix.samples, dtype=np.uint8).reshape(
                pix.height, pix.width, pix.n
            )
            image_bgr = cv2.cvtColor(image_rgb, cv2.COLOR_RGB2BGR)

            yield Unit(
                unit_type="page",
                index=page_index,
                image=image_bgr,
                text_words=words,
            )
    finally:
        doc.close()
