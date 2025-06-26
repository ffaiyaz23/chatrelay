package slack

import "testing"

func TestParseAppMentionText(t *testing.T) {
	botID := "B123"
	cases := []struct {
		text string
		want string
	}{
		{"<@B123> hello world", "hello world"},
		{" <@B123>    multiple   spaces ", "multiple   spaces"},
		{"no mention here", "no mention here"},
	}

	for _, tc := range cases {
		got := ParseAppMentionText(tc.text, botID)
		if got != tc.want {
			t.Errorf("ParseAppMentionText(%q) = %q; want %q", tc.text, got, tc.want)
		}
	}
}
