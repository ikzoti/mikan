package subpage

import (
	"regexp"
	"slices"
	"strings"
)

// The placeholders of an app's link, filled by the page: the subscription's address as it
// is, the same percent-encoded (as encodeURIComponent writes it), and the brand,
// percent-encoded.
const (
	LinkURL    = "{url}"
	LinkURLEnc = "{url_enc}"
	LinkName   = "{name}"
)

// MaxLinkLen is how long an app's link may be.
const MaxLinkLen = 500

// A scheme as RFC 3986 writes it, in lowercase only, of 32 letters at most.
var linkSchemeRe = regexp.MustCompile(`^[a-z][a-z0-9+.-]{0,31}$`)

// refusedSchemes open something other than an app: script in the page, a file, the local
// search or a shell of the visitor's system; any ms- scheme too (Windows hands those to its
// own programs, ms-msdt among them). http goes as well: the subscription's address would
// travel in the clear.
var refusedSchemes = []string{"http", "javascript", "vbscript", "data", "file", "blob", "about", "filesystem", "view-source", "jar", "search-ms", "search", "shell"}

var linkPlaceholders = strings.NewReplacer(LinkURL, "", LinkURLEnc, "", LinkName, "")

// AppLinkProblem says what is wrong with the link that adds the subscription to an app, ""
// when nothing: app_link_invalid (a letter that is not printable ASCII, a link over
// MaxLinkLen, braces that are no placeholder), app_link_scheme (no scheme, one that is not an
// app's, or not in lowercase) or app_link_url (no {url} or {url_enc}). The link is checked
// as it is, nothing trimmed, lowered or decoded, so no spelling of a refused scheme gets
// through: browsers drop tabs and line breaks from a link and read past letters that only
// look Latin, and a link is a URI, whose other letters go %-encoded anyway.
func AppLinkProblem(link string) string {
	if len(link) > MaxLinkLen || strings.ContainsFunc(link, func(r rune) bool { return r <= ' ' || r > '~' }) {
		return "app_link_invalid"
	}
	scheme, _, found := strings.Cut(link, ":")
	if !found || !linkSchemeRe.MatchString(scheme) || strings.HasPrefix(scheme, "ms-") || slices.Contains(refusedSchemes, scheme) {
		return "app_link_scheme"
	}
	if !strings.Contains(link, LinkURL) && !strings.Contains(link, LinkURLEnc) {
		return "app_link_url"
	}
	if strings.ContainsAny(linkPlaceholders.Replace(link), "{}") {
		return "app_link_invalid"
	}
	return ""
}
