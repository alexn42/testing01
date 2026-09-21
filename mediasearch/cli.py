"""Command-line interface: search an image/video/PDF for text or a template image."""

from __future__ import annotations

import argparse
import json
from pathlib import Path
from typing import List, Optional

import cv2

from mediasearch import extract, image_match, text_match
from mediasearch.extract import MediaError


def build_arg_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(
        prog="mediasearch",
        description=(
            "Search an image, video, or PDF for text or a template image, "
            "and print a JSON report of whether (and where) it was found."
        ),
    )
    parser.add_argument(
        "--input", "-i", required=True, help="Path to the image, video, or PDF to search."
    )
    query = parser.add_mutually_exclusive_group(required=True)
    query.add_argument(
        "--text", "-t", help="Text to search for (case-insensitive substring match by default)."
    )
    query.add_argument(
        "--template", help="Path to an image file to search for within the input."
    )
    parser.add_argument(
        "--threshold",
        type=float,
        default=image_match.DEFAULT_THRESHOLD,
        help="Minimum match confidence (0-1) for --template matches. Default: %(default)s.",
    )
    parser.add_argument(
        "--case-sensitive",
        action="store_true",
        help="Require exact case for --text matches (default: case-insensitive).",
    )
    parser.add_argument(
        "--frame-interval",
        type=float,
        default=extract.DEFAULT_FRAME_INTERVAL_SEC,
        help="Seconds between sampled video frames. Default: %(default)s.",
    )
    parser.add_argument(
        "--max-units",
        type=int,
        default=None,
        help="Cap on frames/pages processed (safety limit for long videos or large PDFs).",
    )
    parser.add_argument(
        "--lang",
        default=text_match.DEFAULT_LANG,
        help="Tesseract language code used for OCR. Default: %(default)s.",
    )
    parser.add_argument(
        "--dpi",
        type=int,
        default=extract.DEFAULT_PDF_DPI,
        help="DPI used to render PDF pages before matching. Default: %(default)s.",
    )
    parser.add_argument("--output", "-o", help="Write JSON to this file instead of stdout.")
    parser.add_argument(
        "--compact", action="store_true", help="Emit compact JSON instead of indented JSON."
    )
    return parser


def build_result(
    input_path: str,
    input_type: str,
    query_type: str,
    query_value: str,
    matches_by_unit: List[tuple],
) -> dict:
    matches = []
    for unit, unit_matches in matches_by_unit:
        for m in unit_matches:
            entry = {"unit_type": unit.unit_type, "unit_index": unit.index}
            if unit.timestamp_sec is not None:
                entry["timestamp_sec"] = round(unit.timestamp_sec, 3)
            entry.update(m)
            matches.append(entry)
    return {
        "input": input_path,
        "input_type": input_type,
        "query_type": query_type,
        "query": query_value,
        "found": len(matches) > 0,
        "match_count": len(matches),
        "matches": matches,
    }


def _emit(result: dict, output_path: Optional[str], compact: bool) -> None:
    indent = None if compact else 2
    text = json.dumps(result, indent=indent)
    if output_path:
        Path(output_path).write_text(text + "\n")
    else:
        print(text)


def main(argv: Optional[List[str]] = None) -> int:
    parser = build_arg_parser()
    args = parser.parse_args(argv)

    try:
        # `args.text`/`args.template` are None when the flag was not given at
        # all, but "" when given with an empty value — check `is not None`,
        # not truthiness, or an explicitly empty --text silently falls
        # through to the --template branch.
        if args.text is not None:
            if not args.text:
                raise ValueError("--text must not be empty")
            query_type, query_value, template_image = "text", args.text, None
        else:
            if not args.template:
                raise ValueError("--template must not be empty")
            template_image = cv2.imread(args.template, cv2.IMREAD_COLOR)
            if template_image is None:
                raise MediaError(f"Could not decode template image: {args.template}")
            query_type, query_value = "template_image", args.template

        input_type, units = extract.iter_units(
            args.input,
            frame_interval=args.frame_interval,
            max_units=args.max_units,
            dpi=args.dpi,
        )

        matches_by_unit = []
        for unit in units:
            if query_type == "text":
                unit_matches = text_match.search_text_unit(
                    unit, query_value, case_sensitive=args.case_sensitive, lang=args.lang
                )
            else:
                unit_matches = image_match.search_image_unit(
                    unit, template_image, threshold=args.threshold
                )
            if unit_matches:
                matches_by_unit.append((unit, unit_matches))

        result = build_result(args.input, input_type, query_type, query_value, matches_by_unit)
        _emit(result, args.output, args.compact)
        return 0
    except (MediaError, ValueError) as exc:
        _emit({"input": args.input, "error": str(exc)}, args.output, args.compact)
        return 1
