package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestRunReadsStdinAndEmitsActionableJSON(t *testing.T) {
	input := fixture(t)
	var stdout, stderr bytes.Buffer
	if code := Run(nil, bytes.NewReader(input), &stdout, &stderr); code != 0 {
		t.Fatalf("Run exit code = %d, stderr = %s", code, stderr.String())
	}
	var result document
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("decode JSON output: %v\n%s", err, stdout.String())
	}
	if result.Schema != schema || result.Profile != "cap" || !result.Valid {
		t.Fatalf("unexpected result metadata: %+v", result)
	}
	if result.Alert.Identifier == "" || len(result.Actions) != 1 {
		t.Fatalf("missing actionable CAP data: %+v", result)
	}
	if result.Actions[0].Event == "" || result.Actions[0].Language == "" {
		t.Fatalf("missing action fields: %+v", result.Actions[0])
	}
	if stderr.Len() != 0 {
		t.Fatalf("unexpected stderr: %s", stderr.String())
	}
}

func TestRunSupportsEveryProfile(t *testing.T) {
	input := fixture(t)
	for _, profile := range []string{"cap", "capcp", "ipaws", "nws"} {
		t.Run(profile, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := Run([]string{"-profile", profile, "-compact"}, bytes.NewReader(input), &stdout, &stderr)
			var result document
			if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
				t.Fatalf("decode JSON output: %v\n%s", err, stdout.String())
			}
			if result.Profile != profile || result.Schema != schema {
				t.Fatalf("unexpected profile metadata: %+v", result)
			}
			if profile == "cap" && code != 0 {
				t.Fatalf("base CAP should validate: code=%d stderr=%s", code, stderr.String())
			}
			if profile != "cap" && code == 0 {
				t.Fatalf("fixture should expose profile findings for %s", profile)
			}
		})
	}
}

func TestRunAcceptsFilePath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "alert.xml")
	if err := os.WriteFile(path, fixture(t), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if code := Run([]string{"-profile", "CAP", path}, bytes.NewReader(nil), &stdout, &stderr); code != 0 {
		t.Fatalf("Run exit code = %d, stderr = %s", code, stderr.String())
	}
	var result document
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Profile != "cap" || !result.Valid {
		t.Fatalf("unexpected file result: %+v", result)
	}
}

func TestRunRejectsUnknownProfile(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := Run([]string{"-profile", "unknown"}, bytes.NewReader(nil), &stdout, &stderr); code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("unexpected stdout: %s", stdout.String())
	}
}

func fixture(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "oasis-example.xml"))
	if err != nil {
		t.Fatal(err)
	}
	return data
}
