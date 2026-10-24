package textx

import "testing"

func TestExtractJSON(t *testing.T) {
	for in, want := range map[string]string{
		"Here:\n```json\n{\"a\":[1,2]}\n```": `{"a":[1,2]}`,
		"[{\"a\":1},{\"b\":2}] done":         `[{"a":1},{"b":2}]`,
		"no json":                            "no json",
	} {
		if got := ExtractJSON(in); got != want {
			t.Errorf("%q → %q, harus %q", in, got, want)
		}
	}
}

func TestSnipKeepsRunes(t *testing.T) {
	if got := Snip("héllo wörld", 7); got != "héllo w…" {
		t.Fatalf("got %q", got)
	}
}
