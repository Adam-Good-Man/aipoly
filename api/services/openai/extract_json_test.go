package openai

import "testing"

func TestExtractJSONObjectStripsMarkdownFence(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{`{"summary":"ok"}`, `{"summary":"ok"}`},
		{"```\n{\"summary\":\"ok\"}\n```", `{"summary":"ok"}`},
		{"```json\n{\"summary\":\"ok\"}\n```", `{"summary":"ok"}`},
		{"```{\"summary\":\"ok\"}```", `{"summary":"ok"}`},
		{"Here you go:\n{\"summary\":\"ok\"}\n", `{"summary":"ok"}`},
	}
	for _, tc := range cases {
		got := extractJSONObject(tc.in)
		if got != tc.want {
			t.Fatalf("extractJSONObject(%q)\n got %q\nwant %q", tc.in, got, tc.want)
		}
	}
}
