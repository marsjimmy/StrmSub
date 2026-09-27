package subsource

import "testing"

func TestSanitizeQuery(t *testing.T) {
	cases := map[string]string{
		"Top Gun: Maverick": "Top Gun Maverick",
		"哈利·波特与密室":          "哈利·波特与密室",
		"  金装律师  S01E05 ":   "金装律师 S01E05",
		"V字仇杀队":             "V字仇杀队",
	}
	for in, want := range cases {
		if got := SanitizeQuery(in); got != want {
			t.Errorf("SanitizeQuery(%q)=%q, want %q", in, got, want)
		}
	}
}
