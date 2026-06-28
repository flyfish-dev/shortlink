package server

import "strings"

func normalizeQRPayload(style, fg, bg string) (string, string, string) {
	style = strings.ToLower(strings.TrimSpace(style))
	switch style {
	case "classic", "rounded", "dots":
	default:
		style = "rounded"
	}
	return style, normalizeColor(fg, "#111827"), normalizeColor(bg, "#ffffff")
}

func normalizeColor(v, fallback string) string {
	v = strings.TrimSpace(v)
	if len(v) == 4 && strings.HasPrefix(v, "#") {
		return "#" + strings.Repeat(v[1:2], 2) + strings.Repeat(v[2:3], 2) + strings.Repeat(v[3:4], 2)
	}
	if len(v) != 7 || !strings.HasPrefix(v, "#") {
		return fallback
	}
	for _, r := range v[1:] {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')) {
			return fallback
		}
	}
	return strings.ToLower(v)
}
