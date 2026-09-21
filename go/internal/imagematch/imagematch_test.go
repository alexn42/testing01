package imagematch_test

import (
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"mediasearch/internal/extract"
	"mediasearch/internal/imagematch"
	"mediasearch/internal/testfixture"
)

func sceneUnit(t *testing.T, path string) extract.Unit {
	t.Helper()
	sess, err := extract.Load(path, extract.Options{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { sess.Close() })
	return sess.Units[0]
}

func TestExactCropFoundWithHighConfidence(t *testing.T) {
	dir := t.TempDir()
	scene := testfixture.BuildScene(t, dir)
	unit := sceneUnit(t, scene.ScenePath)

	tmpl, err := imagematch.LoadTemplate(scene.TemplatePath)
	if err != nil {
		t.Fatal(err)
	}
	matches, err := imagematch.SearchImageUnit(unit, tmpl, imagematch.DefaultThreshold, nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 1 {
		t.Fatalf("matches = %d, want 1", len(matches))
	}
	if matches[0].Confidence < 0.95 {
		t.Errorf("confidence = %v, want > 0.95", matches[0].Confidence)
	}
	b := matches[0].BBox
	want := scene.TemplateBBox
	if abs(b.X-want.X) > 5 || abs(b.Y-want.Y) > 5 {
		t.Errorf("bbox = %+v, want near %+v", b, want)
	}
}

func TestFlatColorTemplateNotFalselyMatched(t *testing.T) {
	// Regression test: a naive correlation-based matcher scores a spurious
	// perfect match for a flat/textureless template against any unrelated
	// region, since the correlation coefficient's normalization is 0/0 for
	// zero variance. This implementation avoids the degeneracy entirely by
	// using pixel-color similarity throughout instead of correlation.
	dir := t.TempDir()
	scene := testfixture.BuildScene(t, dir)
	unit := sceneUnit(t, scene.ScenePath)

	tmpl, err := imagematch.LoadTemplate(scene.NegativeTemplatePath)
	if err != nil {
		t.Fatal(err)
	}
	matches, err := imagematch.SearchImageUnit(unit, tmpl, imagematch.DefaultThreshold, nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 0 {
		t.Fatalf("matches = %d, want 0", len(matches))
	}
}

func TestTexturedAbsentTemplateNotMatched(t *testing.T) {
	dir := t.TempDir()
	scene := testfixture.BuildScene(t, dir)
	unit := sceneUnit(t, scene.ScenePath)

	tmpl, err := imagematch.LoadTemplate(scene.TexturedAbsentPath)
	if err != nil {
		t.Fatal(err)
	}
	matches, err := imagematch.SearchImageUnit(unit, tmpl, imagematch.DefaultThreshold, nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 0 {
		t.Fatalf("matches = %d, want 0", len(matches))
	}
}

func TestScaledTemplateStillFound(t *testing.T) {
	dir := t.TempDir()
	scene := testfixture.BuildScene(t, dir)
	unit := sceneUnit(t, scene.ScenePath)

	shrunkPath := filepath.Join(dir, "shrunk_template.png")
	shrinkImage(t, scene.TemplatePath, shrunkPath, 0.75)

	tmpl, err := imagematch.LoadTemplate(shrunkPath)
	if err != nil {
		t.Fatal(err)
	}
	matches, err := imagematch.SearchImageUnit(unit, tmpl, imagematch.DefaultThreshold, nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 1 {
		t.Fatalf("matches = %d, want 1", len(matches))
	}
	if matches[0].Confidence < 0.85 {
		t.Errorf("confidence = %v, want > 0.85", matches[0].Confidence)
	}
}

func TestBelowThresholdMatchNotReported(t *testing.T) {
	dir := t.TempDir()
	scene := testfixture.BuildScene(t, dir)
	unit := sceneUnit(t, scene.ScenePath)

	tmpl, err := imagematch.LoadTemplate(scene.TemplatePath)
	if err != nil {
		t.Fatal(err)
	}
	matches, err := imagematch.SearchImageUnit(unit, tmpl, 1.01, nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 0 {
		t.Fatalf("matches = %d, want 0", len(matches))
	}
}

func shrinkImage(t *testing.T, srcPath, dstPath string, factor float64) {
	t.Helper()
	f, err := os.Open(srcPath)
	if err != nil {
		t.Fatal(err)
	}
	img, _, err := image.Decode(f)
	f.Close()
	if err != nil {
		t.Fatal(err)
	}
	b := img.Bounds()
	nw, nh := int(float64(b.Dx())*factor), int(float64(b.Dy())*factor)
	out := image.NewRGBA(image.Rect(0, 0, nw, nh))
	for y := 0; y < nh; y++ {
		for x := 0; x < nw; x++ {
			out.Set(x, y, img.At(b.Min.X+x*b.Dx()/nw, b.Min.Y+y*b.Dy()/nh))
		}
	}
	of, err := os.Create(dstPath)
	if err != nil {
		t.Fatal(err)
	}
	defer of.Close()
	if err := png.Encode(of, out); err != nil {
		t.Fatal(err)
	}
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
