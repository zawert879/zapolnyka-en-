package cmd

import (
	"strings"
	"testing"
)

func TestPlanUploadsTilde(t *testing.T) {
	plan, err := planUploads([]string{"/x/~logo.png"}, map[string]string{})
	if err != nil {
		t.Fatalf("planUploads: %v", err)
	}
	if plan[0].Ref != "logo.png" || plan[0].UploadName != "logo.png" {
		t.Fatalf("tilde file: ref=%q upload=%q, want logo.png/logo.png", plan[0].Ref, plan[0].UploadName)
	}
}

func TestPlanUploadsUUIDAndStability(t *testing.T) {
	first, err := planUploads([]string{"/x/design.css"}, map[string]string{})
	if err != nil {
		t.Fatalf("planUploads: %v", err)
	}
	up := first[0].UploadName
	if first[0].Ref != "design.css" || up == "design.css" || !strings.HasSuffix(up, ".css") {
		t.Fatalf("uuid file: ref=%q upload=%q", first[0].Ref, up)
	}

	// Re-planning with the recorded manifest must reuse the same upload name.
	second, err := planUploads([]string{"/x/design.css"}, map[string]string{"design.css": up})
	if err != nil {
		t.Fatalf("planUploads(reuse): %v", err)
	}
	if second[0].UploadName != up {
		t.Fatalf("uuid not stable: got %q want %q", second[0].UploadName, up)
	}
}

func TestPlanUploadsCollision(t *testing.T) {
	_, err := planUploads([]string{"/x/logo.png", "/x/~logo.png"}, map[string]string{})
	if err == nil {
		t.Fatal("expected collision error for logo.png vs ~logo.png")
	}
}

func TestExpandPlaceholders(t *testing.T) {
	m := map[string]string{"design.css": "abc.css", "logo.png": "logo.png"}

	got, missing := expandPlaceholders(`@import url("{{design.css}}?v=2");`, m, 82460)
	want := `@import url("https://d1.endata.cx/data/games/82460/abc.css?v=2");`
	if got != want || len(missing) != 0 {
		t.Fatalf("expand: got %q missing %v", got, missing)
	}

	got2, _ := expandPlaceholders("{{ logo.png }}", m, 82460)
	if got2 != "https://d1.endata.cx/data/games/82460/logo.png" {
		t.Fatalf("kept-name expand: %q", got2)
	}

	out, miss := expandPlaceholders("{{nope.png}} and {{design.css}}", m, 82460)
	if len(miss) != 1 || miss[0] != "nope.png" {
		t.Fatalf("missing detection: %v", miss)
	}
	if !strings.Contains(out, "{{nope.png}}") || !strings.Contains(out, "abc.css") {
		t.Fatalf("partial expand: %q", out)
	}
}
