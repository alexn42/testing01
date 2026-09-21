"""Shared fixtures: synthetic image/PDF/video files with known, searchable content."""

import cv2
import numpy as np
import pymupdf
import pytest
from PIL import Image, ImageDraw

SCENE_TEMPLATE_BBOX = {"x": 300, "y": 200, "width": 180, "height": 180}


def _render_text_frame(size, text=None):
    img = Image.new("RGB", size, "white")
    if text:
        ImageDraw.Draw(img).text((40, 100), text, fill="black")
    return cv2.cvtColor(np.array(img), cv2.COLOR_RGB2BGR)


@pytest.fixture
def text_image_path(tmp_path):
    img = Image.new("RGB", (600, 200), "white")
    ImageDraw.Draw(img).text((30, 80), "Hello Mediasearch World", fill="black")
    path = tmp_path / "text_sample.png"
    img.save(path)
    return path


@pytest.fixture
def scene_and_template(tmp_path):
    canvas = Image.new("RGB", (800, 600), "white")
    d = ImageDraw.Draw(canvas)
    d.rectangle([50, 50, 200, 150], fill="red")
    d.ellipse([300, 200, 480, 380], fill="blue")
    d.polygon([(600, 100), (700, 100), (650, 250)], fill="green")
    scene_path = tmp_path / "scene.png"
    canvas.save(scene_path)

    template_path = tmp_path / "template_blue_ellipse.png"
    canvas.crop((300, 200, 480, 380)).save(template_path)

    negative_path = tmp_path / "template_negative.png"
    Image.new("RGB", (150, 150), "yellow").save(negative_path)

    return {
        "scene": scene_path,
        "template": template_path,
        "template_bbox": SCENE_TEMPLATE_BBOX,
        "negative_template": negative_path,
    }


@pytest.fixture
def sample_pdf_path(tmp_path, text_image_path):
    """3-page PDF: two pages of real (digital) text, one page that is only a
    pasted image with no text layer at all -- i.e. a stand-in for a scanned
    page, which forces the OCR fallback path."""
    doc = pymupdf.open()
    doc.new_page().insert_text((72, 72), "The quick brown fox jumps over the lazy dog")
    doc.new_page().insert_text((72, 72), "Nothing interesting is on this page")
    doc.new_page().insert_image(
        pymupdf.Rect(72, 72, 672, 272), filename=str(text_image_path)
    )
    path = tmp_path / "sample.pdf"
    doc.save(str(path))
    doc.close()
    return path


@pytest.fixture
def sample_video_path(tmp_path):
    """10 fps, 50-frame (5s) video; the search text only appears from t=3.0s on."""
    fps = 10
    blank = _render_text_frame((640, 240))
    texted = _render_text_frame((640, 240), "Hello Mediasearch World")

    path = tmp_path / "sample.mp4"
    writer = cv2.VideoWriter(str(path), cv2.VideoWriter_fourcc(*"mp4v"), fps, (640, 240))
    for i in range(50):
        writer.write(texted if i >= 30 else blank)
    writer.release()
    return path
