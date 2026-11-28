package subpage

import (
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Locales are the languages of the page, the panel's own (the web's Locale, web/src/i18n). Each
// text of the admin's may have a translation into each of them, by language in its i18n: the
// page shows the one of its visitor's language, and the text itself where there is none, so the
// text itself stays what the API always had.
var Locales = []string{"ru", "en"}

// BrandText is the line under the brand in one language.
type BrandText struct {
	Subtitle string `json:"subtitle" maxLength:"80"`
}

// BlockText is a block's texts in one language: its heading, and a text block's text.
type BlockText struct {
	Title string `json:"title,omitempty" maxLength:"60"`
	Text  string `json:"text,omitempty" maxLength:"8000" doc:"Только у блока text: Markdown"`
}

// LabelText is the text of a button in one language: a link of a links block, a download.
type LabelText struct {
	Label string `json:"label" maxLength:"40"`
}

// NoteText is the line under an own app's name in one language.
type NoteText struct {
	Note string `json:"note" maxLength:"80"`
}

// DocText is an instruction in one language: its title and its text.
type DocText struct {
	Title string `json:"title,omitempty" maxLength:"80"`
	Body  string `json:"body,omitempty" maxLength:"20000" doc:"Markdown"`
}

// The limits of the admin's texts, the same for a text and its translations.
const (
	maxSubtitle   = 80
	maxBlockTitle = 60
	maxBlockText  = 8000
	maxDocTitle   = 80
	maxDocBody    = 20000
)

// normalizeI18n cleans m, the translations at field: a language the page does not speak is
// refused, clean holds each translation's texts to the rules of the texts they translate (at is
// the path of its fields), and a translation left with no text goes. No translation at all is
// no map, so the document holds only what there is.
func normalizeI18n[T comparable](field string, m map[string]T, clean func(at string, t *T) []Problem) (map[string]T, []Problem) {
	var out []Problem
	var kept map[string]T
	for _, locale := range slices.Sorted(maps.Keys(m)) {
		at := field + "." + locale
		if !slices.Contains(Locales, locale) {
			out = append(out, Problem{Field: at, Code: "value_invalid", Value: locale})
			continue
		}
		t := m[locale]
		out = append(out, clean(at+".", &t)...)
		var none T
		if t == none {
			continue
		}
		if kept == nil {
			kept = make(map[string]T, len(Locales))
		}
		kept[locale] = t
	}
	return kept, out
}

// lineProblem is what is wrong with s as one line of n letters at most, its code and value; ""
// when nothing.
func lineProblem(s string, n int) (string, any) {
	switch {
	case strings.ContainsFunc(s, unicode.IsControl):
		return "one_line", nil
	case utf8.RuneCountInString(s) > n:
		return "too_long", n
	}
	return "", nil
}

// cleanLine trims *s, a line of n letters at most at field, and clears it when it is not one.
func cleanLine(field string, s *string, n int) []Problem {
	*s = strings.TrimSpace(*s)
	code, value := lineProblem(*s, n)
	if code == "" {
		return nil
	}
	*s = ""
	return []Problem{{Field: field, Code: code, Value: value}}
}

// cleanMarkdown trims *s, a Markdown text of n letters at most at field, with its line breaks as
// \n. One too long, or with a control character PostgreSQL or the page would choke on, is
// cleared; an image that is not https is said, and the page leaves it out.
func cleanMarkdown(field string, s *string, n int) []Problem {
	*s = strings.TrimSpace(strings.ReplaceAll(*s, "\r\n", "\n"))
	switch {
	case utf8.RuneCountInString(*s) > n:
		*s = ""
		return []Problem{{Field: field, Code: "too_long", Value: n}}
	case HasControl(*s):
		*s = ""
		return []Problem{{Field: field, Code: "value_invalid"}}
	}
	if u := FirstBadImage(*s); u != "" {
		return []Problem{{Field: field, Code: "image_https", Value: u}}
	}
	return nil
}

func cleanBrandText(at string, t *BrandText) []Problem {
	return cleanLine(at+"subtitle", &t.Subtitle, maxSubtitle)
}

// cleanBlockText is how the translations of a block of type typ are cleaned: the heading as a
// heading, the text as a text block's. Another block has no text, so none is translated.
func cleanBlockText(typ string) func(string, *BlockText) []Problem {
	return func(at string, t *BlockText) []Problem {
		out := cleanLine(at+"title", &t.Title, maxBlockTitle)
		if typ == TypeText {
			return append(out, cleanMarkdown(at+"text", &t.Text, maxBlockText)...)
		}
		if strings.TrimSpace(t.Text) != "" {
			out = append(out, Problem{Field: at + "text", Code: "value_invalid"})
		}
		t.Text = ""
		return out
	}
}

func cleanLabelText(at string, t *LabelText) []Problem {
	return cleanLine(at+"label", &t.Label, maxLabel)
}

func cleanNoteText(at string, t *NoteText) []Problem {
	return cleanLine(at+"note", &t.Note, maxNote)
}

func cleanDocText(at string, t *DocText) []Problem {
	out := cleanLine(at+"title", &t.Title, maxDocTitle)
	return append(out, cleanMarkdown(at+"body", &t.Body, maxDocBody)...)
}

// MarshalDocI18n is how the database keeps the translations of an instruction
// (sub_docs.translations): JSON by language, {} for none.
func MarshalDocI18n(m map[string]DocText) (string, error) {
	if len(m) == 0 {
		return "{}", nil
	}
	b, err := json.Marshal(m)
	return string(b), err
}

// MarshalDocTitles is how the database keeps the titles of an instruction's translations apart
// from their texts (sub_docs.translated_titles): the page's list reads them on every view, and
// never a text.
func MarshalDocTitles(m map[string]DocText) (string, error) {
	titles := make(map[string]DocText, len(m))
	for locale, t := range m {
		if t.Title != "" {
			titles[locale] = DocText{Title: t.Title}
		}
	}
	return MarshalDocI18n(titles)
}

// UnmarshalDocI18n reads the translations of an instruction as the database keeps them, whole or
// their titles alone, cleaned as NormalizeDoc cleans them. A value that does not read gives none,
// with ErrUnreadable: the instruction is shown in its own words rather than not at all.
func UnmarshalDocI18n(raw string) (map[string]DocText, error) {
	var m map[string]DocText
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		return nil, fmt.Errorf("%w: translations: %w", ErrUnreadable, err)
	}
	m, _ = normalizeI18n("i18n", m, cleanDocText)
	return m, nil
}
