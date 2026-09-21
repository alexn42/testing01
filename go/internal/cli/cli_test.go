package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"mediasearch/internal/cli"
	"mediasearch/internal/testfixture"
)

// runCLI always routes through --output rather than capturing stdout, since
// that exercises the exact same execute()/JSON-building path -- the sink is
// the only thing that differs, and the stdout path is covered by manual
// end-to-end runs of the compiled binary.
func runCLI(t *testing.T, args []string) (int, map[string]interface{}) {
	t.Helper()
	outPath := filepath.Join(t.TempDir(), "out.json")
	fullArgs := append(append([]string{}, args...), "--output", outPath, "--compact")
	code := cli.Main(fullArgs)

	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("no output written (exit=%d): %v", code, err)
	}
	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("invalid JSON output: %v\n%s", err, data)
	}
	return code, result
}

func TestTextFoundEndToEnd(t *testing.T) {
	path := testfixture.TextImage(t, t.TempDir(), "Hello Mediasearch World")
	code, result := runCLI(t, []string{"--input", path, "--text", "hello"})
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	if result["found"] != true {
		t.Errorf("found = %v, want true", result["found"])
	}
	if result["query_type"] != "text" {
		t.Errorf("query_type = %v, want text", result["query_type"])
	}
	if result["input_type"] != "image" {
		t.Errorf("input_type = %v, want image", result["input_type"])
	}
}

func TestTextNotFoundStillExitsZeroWithValidJSON(t *testing.T) {
	path := testfixture.TextImage(t, t.TempDir(), "Hello Mediasearch World")
	code, result := runCLI(t, []string{"--input", path, "--text", "nonexistent"})
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	if result["found"] != false {
		t.Errorf("found = %v, want false", result["found"])
	}
	matches, ok := result["matches"].([]interface{})
	if !ok || len(matches) != 0 {
		t.Errorf("matches = %v, want []", result["matches"])
	}
}

func TestTemplateFoundEndToEnd(t *testing.T) {
	dir := t.TempDir()
	scene := testfixture.BuildScene(t, dir)
	code, result := runCLI(t, []string{"--input", scene.ScenePath, "--template", scene.TemplatePath})
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	if result["found"] != true {
		t.Errorf("found = %v, want true", result["found"])
	}
	if result["query_type"] != "template_image" {
		t.Errorf("query_type = %v, want template_image", result["query_type"])
	}
}

func TestMissingInputFileErrorsCleanly(t *testing.T) {
	code, result := runCLI(t, []string{"--input", filepath.Join(t.TempDir(), "nope.png"), "--text", "x"})
	if code != 1 {
		t.Fatalf("exit = %d, want 1", code)
	}
	if result["error"] == nil {
		t.Errorf("expected an \"error\" field, got %v", result)
	}
}

func TestUnsupportedExtensionErrorsCleanly(t *testing.T) {
	dir := t.TempDir()
	bogus := filepath.Join(dir, "notes.txt")
	if err := os.WriteFile(bogus, []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}
	code, result := runCLI(t, []string{"--input", bogus, "--text", "x"})
	if code != 1 {
		t.Fatalf("exit = %d, want 1", code)
	}
	if result["error"] == nil {
		t.Errorf("expected an \"error\" field, got %v", result)
	}
}

func TestEmptyTextValueErrorsCleanlyRatherThanMisrouting(t *testing.T) {
	// Regression test: flag's zero value for an unset string flag is "",
	// indistinguishable from an explicitly empty --text by value alone.
	// Code that didn't use fs.Visit to check "was --text actually given"
	// would silently fall through to the --template branch instead.
	path := testfixture.TextImage(t, t.TempDir(), "Hello Mediasearch World")
	code, result := runCLI(t, []string{"--input", path, "--text", ""})
	if code != 1 {
		t.Fatalf("exit = %d, want 1", code)
	}
	if result["error"] != "--text must not be empty" {
		t.Errorf("error = %v, want '--text must not be empty'", result["error"])
	}
}

func TestEmptyTemplateValueErrorsCleanly(t *testing.T) {
	path := testfixture.TextImage(t, t.TempDir(), "Hello Mediasearch World")
	code, result := runCLI(t, []string{"--input", path, "--template", ""})
	if code != 1 {
		t.Fatalf("exit = %d, want 1", code)
	}
	if result["error"] != "--template must not be empty" {
		t.Errorf("error = %v, want '--template must not be empty'", result["error"])
	}
}

func TestTextAndTemplateAreMutuallyExclusive(t *testing.T) {
	path := testfixture.TextImage(t, t.TempDir(), "Hello Mediasearch World")
	code := cli.Main([]string{"--input", path, "--text", "x", "--template", path})
	if code != 2 {
		t.Fatalf("exit = %d, want 2 (usage error)", code)
	}
}

func TestNeitherTextNorTemplateIsUsageError(t *testing.T) {
	path := testfixture.TextImage(t, t.TempDir(), "Hello Mediasearch World")
	code := cli.Main([]string{"--input", path})
	if code != 2 {
		t.Fatalf("exit = %d, want 2 (usage error)", code)
	}
}

func TestMissingInputFlagIsUsageError(t *testing.T) {
	code := cli.Main([]string{"--text", "x"})
	if code != 2 {
		t.Fatalf("exit = %d, want 2 (usage error)", code)
	}
}

func TestCaseSensitiveFlagIsRespected(t *testing.T) {
	path := testfixture.TextImage(t, t.TempDir(), "Hello Mediasearch World")
	code, result := runCLI(t, []string{"--input", path, "--text", "MEDIASEARCH", "--case-sensitive"})
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	if result["found"] != false {
		t.Errorf("found = %v, want false (wrong case)", result["found"])
	}
}
