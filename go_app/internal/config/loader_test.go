package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTemp(t *testing.T, name, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestLoadCodesErrorPointsAtFileAndLine(t *testing.T) {
	p := writeTemp(t, "codes.yml", `# комментарий
- type: сектор
  sectorName: "s"
  answers: [a]

- type: бонус
  bonusName: "b"
  answers: [b]
`)
	_, err := LoadCodes(p)
	if err == nil {
		t.Fatal("expected error")
	}
	msg := err.Error()
	for _, want := range []string{p + ":6", "запись 2", "type=бонус", "time"} {
		if !strings.Contains(msg, want) {
			t.Errorf("error %q should contain %q", msg, want)
		}
	}
}

func TestLoadCodesUnknownType(t *testing.T) {
	p := writeTemp(t, "codes.yml", "- type: бонусы\n  answers: [a]\n  time: 1\n")
	_, err := LoadCodes(p)
	if err == nil || !strings.Contains(err.Error(), "неизвестный type") || !strings.Contains(err.Error(), p+":1") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadCodesJSONWithoutLines(t *testing.T) {
	p := writeTemp(t, "codes.json", `[{"type":"бонус","answers":["a"]}]`)
	_, err := LoadCodes(p)
	if err == nil || !strings.Contains(err.Error(), p+" (запись 1)") {
		t.Fatalf("unexpected error: %v", err)
	}
}
