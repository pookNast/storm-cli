package llm

import "testing"

func TestExtractJSON(t *testing.T) {
	cases := []struct{ in, want string }{
		{`{"a":1}`, `{"a":1}`},
		{"```json\n{\"a\":1}\n```", `{"a":1}`},
		{"```\n{\"a\":1}\n```", `{"a":1}`},
		{"The result is:\n{\"a\":1}\nDone.", `{"a":1}`},
		{"Here you go: {\"a\":1}", `{"a":1}`},
	}
	for _, tc := range cases {
		got := ExtractJSON(tc.in)
		if got != tc.want {
			t.Errorf("ExtractJSON(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
