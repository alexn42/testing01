package extract_test

import (
	"os"
	"path/filepath"
	"testing"

	"mediasearch/internal/extract"
	"mediasearch/internal/testfixture"
)

func TestImageYieldsOneUnit(t *testing.T) {
	dir := t.TempDir()
	path := testfixture.TextImage(t, dir, "Hello Mediasearch World")

	sess, err := extract.Load(path, extract.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer sess.Close()

	if sess.InputType != "image" {
		t.Errorf("InputType = %q, want image", sess.InputType)
	}
	if len(sess.Units) != 1 {
		t.Fatalf("len(Units) = %d, want 1", len(sess.Units))
	}
	if sess.Units[0].UnitType != "image" || sess.Units[0].Index != 0 {
		t.Errorf("unexpected unit: %+v", sess.Units[0])
	}
}

func TestMissingFileErrors(t *testing.T) {
	_, err := extract.Load(filepath.Join(t.TempDir(), "nope.png"), extract.Options{})
	if err == nil {
		t.Fatal("expected an error for a missing file")
	}
}

func TestUnsupportedExtensionErrors(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "notes.txt")
	if err := os.WriteFile(path, []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := extract.Load(path, extract.Options{})
	if err == nil {
		t.Fatal("expected an error for an unsupported extension")
	}
}

func TestPDFPagesAndTextLayer(t *testing.T) {
	sess, err := extract.Load(testfixture.SamplePDFPath(t), extract.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer sess.Close()

	if sess.InputType != "pdf" {
		t.Errorf("InputType = %q, want pdf", sess.InputType)
	}
	if len(sess.Units) != 3 {
		t.Fatalf("len(Units) = %d, want 3", len(sess.Units))
	}
	if len(sess.Units[0].TextWords) == 0 {
		t.Error("page 0 should have a digital text layer")
	}
	if sess.Units[2].TextWords != nil {
		t.Errorf("page 2 (image-only) should have TextWords == nil, got %d words", len(sess.Units[2].TextWords))
	}
}

func TestPDFMaxUnitsCapsPages(t *testing.T) {
	sess, err := extract.Load(testfixture.SamplePDFPath(t), extract.Options{MaxUnits: 1})
	if err != nil {
		t.Fatal(err)
	}
	defer sess.Close()
	if len(sess.Units) != 1 {
		t.Fatalf("len(Units) = %d, want 1", len(sess.Units))
	}
}

func TestVideoFrameSampling(t *testing.T) {
	dir := t.TempDir()
	path := testfixture.BuildVideo(t, dir, 10, 30, 20, "Hello Mediasearch World")

	sess, err := extract.Load(path, extract.Options{FrameInterval: 1.0})
	if err != nil {
		t.Fatal(err)
	}
	defer sess.Close()

	if len(sess.Units) != 5 {
		t.Fatalf("len(Units) = %d, want 5", len(sess.Units))
	}
	want := []float64{0, 1, 2, 3, 4}
	for i, u := range sess.Units {
		if u.TimestampSec == nil || *u.TimestampSec != want[i] {
			t.Errorf("unit %d timestamp = %v, want %v", i, u.TimestampSec, want[i])
		}
	}
}

func TestVideoFrameIntervalAndMaxUnits(t *testing.T) {
	dir := t.TempDir()
	path := testfixture.BuildVideo(t, dir, 10, 30, 20, "Hello Mediasearch World")

	sess, err := extract.Load(path, extract.Options{FrameInterval: 2.0, MaxUnits: 2})
	if err != nil {
		t.Fatal(err)
	}
	defer sess.Close()

	if len(sess.Units) != 2 {
		t.Fatalf("len(Units) = %d, want 2", len(sess.Units))
	}
	want := []float64{0, 2}
	for i, u := range sess.Units {
		if u.TimestampSec == nil || *u.TimestampSec != want[i] {
			t.Errorf("unit %d timestamp = %v, want %v", i, u.TimestampSec, want[i])
		}
	}
}
