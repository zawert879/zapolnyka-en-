package assets

import (
	"strings"
	"testing"
	"zapolnyaka/internal/config"
)

func TestPlanUploadsTilde(t *testing.T) {
	plan, err := PlanUploads([]string{"/x/~logo.png"}, map[string]string{})
	if err != nil {
		t.Fatalf("PlanUploads: %v", err)
	}
	if plan[0].Ref != "logo.png" || plan[0].UploadName != "logo.png" {
		t.Fatalf("tilde file: ref=%q upload=%q, want logo.png/logo.png", plan[0].Ref, plan[0].UploadName)
	}
}

func TestPlanUploadsUUIDAndStability(t *testing.T) {
	first, err := PlanUploads([]string{"/x/design.css"}, map[string]string{})
	if err != nil {
		t.Fatalf("PlanUploads: %v", err)
	}
	up := first[0].UploadName
	if first[0].Ref != "design.css" || up == "design.css" || !strings.HasSuffix(up, ".css") {
		t.Fatalf("uuid file: ref=%q upload=%q", first[0].Ref, up)
	}

	// Повторное планирование с записанным манифестом должно переиспользовать имя.
	second, err := PlanUploads([]string{"/x/design.css"}, map[string]string{"design.css": up})
	if err != nil {
		t.Fatalf("PlanUploads(reuse): %v", err)
	}
	if second[0].UploadName != up {
		t.Fatalf("uuid not stable: got %q want %q", second[0].UploadName, up)
	}
}

func TestPlanUploadsCollision(t *testing.T) {
	_, err := PlanUploads([]string{"/x/logo.png", "/x/~logo.png"}, map[string]string{})
	if err == nil {
		t.Fatal("expected collision error for logo.png vs ~logo.png")
	}
}

func TestExpandD1(t *testing.T) {
	m := map[string]string{"design.css": "abc.css", "logo.png": "logo.png"}
	r := D1Resolver(m, 82460)

	got, missing := Expand(`@import url("{{design.css}}?v=2");`, r)
	want := `@import url("https://d1.endata.cx/data/games/82460/abc.css?v=2");`
	if got != want || len(missing) != 0 {
		t.Fatalf("expand: got %q missing %v", got, missing)
	}

	got2, _ := Expand("{{ logo.png }}", r)
	if got2 != "https://d1.endata.cx/data/games/82460/logo.png" {
		t.Fatalf("kept-name expand: %q", got2)
	}

	out, miss := Expand("{{nope.png}} and {{design.css}}", r)
	if len(miss) != 1 || miss[0] != "nope.png" {
		t.Fatalf("missing detection: %v", miss)
	}
	if !strings.Contains(out, "{{nope.png}}") || !strings.Contains(out, "abc.css") {
		t.Fatalf("partial expand: %q", out)
	}
}

func TestReverse(t *testing.T) {
	r := Reverse(map[string]string{"design.css": "abc.css"})
	if r["abc.css"] != "design.css" {
		t.Fatalf("reverse: %v", r)
	}
}

func TestSubstituteCodeTaskAndHelp(t *testing.T) {
	r := D1Resolver(map[string]string{"1.jpg": "abc.jpg"}, 82460)
	task := `<img src="{{1.jpg}}">`
	help := "ok {{1.jpg}}"
	prepared := []config.PreparedLevel{{
		Codes: []config.Code{{Type: config.CodeTypeBonus, Task: &task, Help: &help, Answers: []string{"x"}}},
	}}
	if err := Substitute(prepared, r); err != nil {
		t.Fatalf("Substitute: %v", err)
	}
	wantURL := "https://d1.endata.cx/data/games/82460/abc.jpg"
	if got := *prepared[0].Codes[0].Task; got != `<img src="`+wantURL+`">` {
		t.Fatalf("task: %q", got)
	}
	if got := *prepared[0].Codes[0].Help; got != "ok "+wantURL {
		t.Fatalf("help: %q", got)
	}

	missingTask := "{{nope.png}}"
	prepared[0].Codes[0].Task = &missingTask
	if err := Substitute(prepared, r); err == nil {
		t.Fatalf("expected error for missing placeholder in task")
	}
}
