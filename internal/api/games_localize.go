package api

import "strings"

// NormalizeGameListLang maps query param lang to zh | en | ur; unknown or empty defaults to zh.
func NormalizeGameListLang(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "en", "english":
		return "en"
	case "ur", "urdu":
		return "ur"
	default:
		return "zh"
	}
}

// LocalizeGameItem copies g and sets Name/Note to the requested language; clears nameEn/nameUr/noteEn/noteUr
// so JSON omitempty hides multilingual source fields from clients.
func LocalizeGameItem(g GameItem, lang string) GameItem {
	out := g
	out.Name = pickLocalizedText(g.Name, g.NameEn, g.NameUr, lang)
	out.Note = pickLocalizedText(g.Note, g.NoteEn, g.NoteUr, lang)
	out.NameEn = ""
	out.NameUr = ""
	out.NoteEn = ""
	out.NoteUr = ""
	return out
}

func pickLocalizedText(zh, en, ur, lang string) string {
	zh = strings.TrimSpace(zh)
	en = strings.TrimSpace(en)
	ur = strings.TrimSpace(ur)
	switch lang {
	case "en":
		return firstNonEmpty(en, zh, ur)
	case "ur":
		return firstNonEmpty(ur, en, zh)
	default:
		return firstNonEmpty(zh, en, ur)
	}
}

func firstNonEmpty(a, b, c string) string {
	if a != "" {
		return a
	}
	if b != "" {
		return b
	}
	return c
}
