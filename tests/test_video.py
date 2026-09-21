from mediasearch import extract, text_match


def test_frame_sampling_uses_requested_interval(sample_video_path):
    # fps=10, 50 frames => 5s of video; frame_interval=1.0 => 5 samples.
    _, units = extract.iter_units(sample_video_path, frame_interval=1.0)
    timestamps = [u.timestamp_sec for u in units]
    assert timestamps == [0.0, 1.0, 2.0, 3.0, 4.0]


def test_wider_frame_interval_samples_fewer_frames(sample_video_path):
    _, units = extract.iter_units(sample_video_path, frame_interval=2.0)
    timestamps = [u.timestamp_sec for u in units]
    assert timestamps == [0.0, 2.0, 4.0]


def test_max_units_caps_sampled_frames(sample_video_path):
    _, units = extract.iter_units(sample_video_path, frame_interval=1.0, max_units=2)
    assert len(list(units)) == 2


def test_text_only_found_in_frames_after_it_appears(sample_video_path):
    # The fixture only draws the search text from t=3.0s onward.
    _, units = extract.iter_units(sample_video_path, frame_interval=1.0)
    found_at = [
        u.timestamp_sec for u in units if text_match.search_text_unit(u, "mediasearch")
    ]
    assert found_at == [3.0, 4.0]


def test_text_never_present_is_never_found(sample_video_path):
    _, units = extract.iter_units(sample_video_path, frame_interval=1.0)
    assert all(text_match.search_text_unit(u, "goodbye") == [] for u in units)
