import json

import pytest

from mediasearch.cli import main


def _run(capsys, argv):
    exit_code = main(argv)
    return exit_code, json.loads(capsys.readouterr().out)


def test_text_found_end_to_end(capsys, text_image_path):
    exit_code, result = _run(capsys, ["--input", str(text_image_path), "--text", "hello", "--compact"])
    assert exit_code == 0
    assert result["found"] is True
    assert result["query_type"] == "text"
    assert result["input_type"] == "image"
    assert result["match_count"] == 1


def test_text_not_found_still_exits_zero_with_valid_json(capsys, text_image_path):
    exit_code, result = _run(
        capsys, ["--input", str(text_image_path), "--text", "nonexistent", "--compact"]
    )
    assert exit_code == 0
    assert result["found"] is False
    assert result["matches"] == []


def test_template_found_end_to_end(capsys, scene_and_template):
    exit_code, result = _run(
        capsys,
        [
            "--input", str(scene_and_template["scene"]),
            "--template", str(scene_and_template["template"]),
            "--compact",
        ],
    )
    assert exit_code == 0
    assert result["found"] is True
    assert result["query_type"] == "template_image"


def test_missing_input_file_errors_cleanly(capsys, tmp_path):
    exit_code, result = _run(
        capsys, ["--input", str(tmp_path / "nope.png"), "--text", "x", "--compact"]
    )
    assert exit_code == 1
    assert "error" in result


def test_unsupported_extension_errors_cleanly(capsys, tmp_path):
    bogus = tmp_path / "notes.txt"
    bogus.write_text("hello")
    exit_code, result = _run(capsys, ["--input", str(bogus), "--text", "x", "--compact"])
    assert exit_code == 1
    assert "error" in result


def test_empty_text_value_errors_cleanly_rather_than_misrouting(capsys, text_image_path):
    # Regression test: argparse sets args.text = "" (falsy, but *provided*)
    # for `--text ""`. Code that checks truthiness instead of `is not None`
    # would misroute this to the --template branch instead of reporting a
    # clear "--text must not be empty" error.
    exit_code, result = _run(capsys, ["--input", str(text_image_path), "--text", "", "--compact"])
    assert exit_code == 1
    assert result["error"] == "--text must not be empty"


def test_text_and_template_are_mutually_exclusive(text_image_path, scene_and_template):
    with pytest.raises(SystemExit):
        main(
            [
                "--input", str(text_image_path),
                "--text", "hello",
                "--template", str(scene_and_template["template"]),
            ]
        )


def test_output_flag_writes_json_to_file(tmp_path, text_image_path):
    out_path = tmp_path / "out.json"
    exit_code = main(
        [
            "--input", str(text_image_path),
            "--text", "hello",
            "--output", str(out_path),
            "--compact",
        ]
    )
    assert exit_code == 0
    result = json.loads(out_path.read_text())
    assert result["found"] is True
