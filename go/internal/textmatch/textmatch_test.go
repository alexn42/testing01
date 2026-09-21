package textmatch_test

import (
	"testing"

	"mediasearch/internal/extract"
	"mediasearch/internal/testfixture"
	"mediasearch/internal/textmatch"
)

func firstUnit(t *testing.T, path string) extract.Unit {
	t.Helper()
	sess, err := extract.Load(path, extract.Options{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { sess.Close() })
	if len(sess.Units) == 0 {
		t.Fatal("loaded session has no units")
	}
	return sess.Units[0]
}

func TestFindsSubstringCaseInsensitive(t *testing.T) {
	unit := firstUnit(t, testfixture.TextImage(t, t.TempDir(), "Hello Mediasearch World"))
	matches, err := textmatch.SearchTextUnit(unit, "mediasearch", false, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 1 {
		t.Fatalf("matches = %d, want 1", len(matches))
	}
	if matches[0].Source != "ocr" {
		t.Errorf("source = %q, want ocr", matches[0].Source)
	}
	if matches[0].Confidence <= 0 || matches[0].Confidence > 1 {
		t.Errorf("confidence = %v, want in (0,1]", matches[0].Confidence)
	}
}

func TestAbsentTextNotFound(t *testing.T) {
	unit := firstUnit(t, testfixture.TextImage(t, t.TempDir(), "Hello Mediasearch World"))
	matches, err := textmatch.SearchTextUnit(unit, "goodbye", false, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 0 {
		t.Fatalf("matches = %d, want 0", len(matches))
	}
}

func TestCaseSensitiveRejectsWrongCase(t *testing.T) {
	unit := firstUnit(t, testfixture.TextImage(t, t.TempDir(), "Hello Mediasearch World"))
	if m, err := textmatch.SearchTextUnit(unit, "MEDIASEARCH", true, ""); err != nil || len(m) != 0 {
		t.Errorf("wrong-case query: matches=%d err=%v, want 0 matches", len(m), err)
	}
	if m, err := textmatch.SearchTextUnit(unit, "Mediasearch", true, ""); err != nil || len(m) == 0 {
		t.Errorf("correct-case query: matches=%d err=%v, want >=1 match", len(m), err)
	}
}

func TestEmptyQueryIsRejected(t *testing.T) {
	unit := firstUnit(t, testfixture.TextImage(t, t.TempDir(), "Hello Mediasearch World"))
	if _, err := textmatch.SearchTextUnit(unit, "", false, ""); err == nil {
		t.Fatal("expected an error for an empty query")
	}
}

func TestPDFDigitalTextLayerUsedForNormalPages(t *testing.T) {
	unit := firstUnit(t, testfixture.SamplePDFPath(t))
	matches, err := textmatch.SearchTextUnit(unit, "quick brown fox", false, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 1 {
		t.Fatalf("matches = %d, want 1", len(matches))
	}
	if matches[0].Source != "pdf_text" {
		t.Errorf("source = %q, want pdf_text", matches[0].Source)
	}
	if matches[0].Confidence != 1.0 {
		t.Errorf("confidence = %v, want 1.0 (exact digital text)", matches[0].Confidence)
	}
}

func TestPDFOCRFallbackForImageOnlyPage(t *testing.T) {
	sess, err := extract.Load(testfixture.SamplePDFPath(t), extract.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer sess.Close()

	page3 := sess.Units[2]
	matches, err := textmatch.SearchTextUnit(page3, "mediasearch", false, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 1 {
		t.Fatalf("matches = %d, want 1", len(matches))
	}
	if matches[0].Source != "ocr" {
		t.Errorf("source = %q, want ocr", matches[0].Source)
	}
}

func TestPDFMatchNotReportedOnWrongPage(t *testing.T) {
	sess, err := extract.Load(testfixture.SamplePDFPath(t), extract.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer sess.Close()

	if m, err := textmatch.SearchTextUnit(sess.Units[1], "quick brown fox", false, ""); err != nil || len(m) != 0 {
		t.Errorf("page 1: matches=%d err=%v, want 0 matches", len(m), err)
	}
	if m, err := textmatch.SearchTextUnit(sess.Units[0], "nothing interesting", false, ""); err != nil || len(m) != 0 {
		t.Errorf("page 0: matches=%d err=%v, want 0 matches", len(m), err)
	}
}

func TestPDFTextAbsentFromEveryPage(t *testing.T) {
	sess, err := extract.Load(testfixture.SamplePDFPath(t), extract.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer sess.Close()

	for _, unit := range sess.Units {
		if m, err := textmatch.SearchTextUnit(unit, "nonexistent phrase xyz", false, ""); err != nil || len(m) != 0 {
			t.Errorf("unit %d: matches=%d err=%v, want 0 matches", unit.Index, len(m), err)
		}
	}
}

func TestVideoTextOnlyFoundAfterItAppears(t *testing.T) {
	dir := t.TempDir()
	path := testfixture.BuildVideo(t, dir, 10, 30, 20, "Hello Mediasearch World")

	sess, err := extract.Load(path, extract.Options{FrameInterval: 1.0})
	if err != nil {
		t.Fatal(err)
	}
	defer sess.Close()

	var foundAt []float64
	for _, u := range sess.Units {
		if m, err := textmatch.SearchTextUnit(u, "mediasearch", false, ""); err != nil {
			t.Fatal(err)
		} else if len(m) > 0 {
			foundAt = append(foundAt, *u.TimestampSec)
		}
	}
	want := []float64{3, 4}
	if len(foundAt) != len(want) || foundAt[0] != want[0] || foundAt[1] != want[1] {
		t.Errorf("foundAt = %v, want %v", foundAt, want)
	}
}
