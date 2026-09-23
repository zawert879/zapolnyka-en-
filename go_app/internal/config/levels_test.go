package config

import (
	"reflect"
	"testing"
)

func TestParseLevelSpec(t *testing.T) {
	game := []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 15}
	cases := []struct {
		spec string
		want []int
	}{
		{"7", []int{7}},
		{"1-3", []int{1, 2, 3}},
		{">10", []int{11, 12, 15}},
		{">=10", []int{10, 11, 12, 15}},
		{"<3", []int{1, 2}},
		{"<=3", []int{1, 2, 3}},
		{"2, 4, 6-8", []int{2, 4, 6, 7, 8}},
		{"<=2, >=12", []int{1, 2, 12, 15}},
		{" 1 - 2 ", []int{1, 2}},
		{"*", game},
		{"все", game},
		{"<1", nil},
	}
	for _, c := range cases {
		f, err := ParseLevelSpec(c.spec)
		if err != nil {
			t.Fatalf("%q: unexpected error: %v", c.spec, err)
		}
		if got := f.Resolve(game); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%q: got %v, want %v", c.spec, got, c.want)
		}
	}

	for _, bad := range []string{"", "  ", ",", "abc", "5-3", "0", "-2", ">x", "1-", "3-abc"} {
		if _, err := ParseLevelSpec(bad); err == nil {
			t.Errorf("%q: expected error", bad)
		}
	}
}
