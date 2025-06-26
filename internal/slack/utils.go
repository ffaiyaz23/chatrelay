package slack

import "strings"

// ParseAppMentionText strips the leading "<@BOTID>" mention and returns the rest.
//
// For example, given text "<@B123> hello world" and botID "B123",
// it returns "hello world".
func ParseAppMentionText(text, botID string) string {
	prefix := "<@" + botID + ">"
	trimmed := strings.TrimSpace(text)
	if strings.HasPrefix(trimmed, prefix) {
		// Slice off the prefix length, then trim spaces again
		return strings.TrimSpace(trimmed[len(prefix):])
	}
	return trimmed
}
