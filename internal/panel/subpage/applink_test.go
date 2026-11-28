package subpage

import (
	"strings"
	"testing"
)

func TestAppLinkProblem(t *testing.T) {
	long := func(n int) string { return "myapp://add/{url}#" + strings.Repeat("a", n-len("myapp://add/{url}#")) }
	for _, c := range []struct {
		link, want string
	}{
		{"myapp://add/{url}", ""},
		{"myapp://install-config?url={url_enc}&name={name}", ""},
		{"myapp://import/{url}#{name}", ""},
		{"myapp://{url}{url_enc}{name}", ""},
		{"com.example.vpn:{url}", ""},
		{"my-app+x.y:{url_enc}", ""},
		{"intent://add/{url}#Intent;scheme=myapp;end", ""},
		{"https://example.com/add?u={url_enc}", ""},
		{"a:{url}", ""},
		{strings.Repeat("a", 32) + ":{url}", ""},
		{long(500), ""},

		// The scheme opens something other than an app: refused under every spelling.
		{"javascript:alert(1)//{url}", "app_link_scheme"},
		{"JavaScript:alert(1)//{url}", "app_link_scheme"},
		{"JAVASCRIPT:alert(1)//{url}", "app_link_scheme"},
		{"java%73cript:alert(1)//{url}", "app_link_scheme"},
		{"%6Aavascript:alert(1)//{url}", "app_link_scheme"},
		{"javascript&colon;alert(1)//{url}", "app_link_scheme"},
		{"jav&#x61;script:alert(1)//{url}", "app_link_scheme"},
		{"vbscript:msgbox(1)//{url}", "app_link_scheme"},
		{"data:text/html,{url}", "app_link_scheme"},
		{"file:///C:/{url}", "app_link_scheme"},
		{"blob:https://example.com/{url}", "app_link_scheme"},
		{"about:blank#{url}", "app_link_scheme"},
		{"filesystem:https://example.com/{url}", "app_link_scheme"},
		{"view-source:https://example.com/{url}", "app_link_scheme"},
		{"jar:file:///x.jar!/{url}", "app_link_scheme"},
		{"search-ms:query={url}", "app_link_scheme"},
		{"search:query={url}", "app_link_scheme"},
		{"shell:startup/{url}", "app_link_scheme"},
		{"ms-msdt:/id/PCWDiagnostic/{url}", "app_link_scheme"},
		{"ms-officecmd:{url}", "app_link_scheme"},
		{"ms-settings:{url}", "app_link_scheme"},
		{"ms-:{url}", "app_link_scheme"},
		{"http://example.com/{url}", "app_link_scheme"},
		{"HTTP://example.com/{url}", "app_link_scheme"},
		{"Https://example.com/{url}", "app_link_scheme"},
		{"Happ://add/{url}", "app_link_scheme"},
		{"1app:{url}", "app_link_scheme"},
		{"my_app:{url}", "app_link_scheme"},
		{"-app:{url}", "app_link_scheme"},
		{strings.Repeat("a", 33) + ":{url}", "app_link_scheme"},
		{":{url}", "app_link_scheme"},
		{"//example.com/{url}", "app_link_scheme"},
		{"{url}", "app_link_scheme"},
		{"{url}:x", "app_link_scheme"},
		{"", "app_link_scheme"},

		// What a browser drops or reads past is refused, not cleaned into a scheme.
		{" javascript:alert(1)//{url}", "app_link_invalid"},
		{"\tjavascript:alert(1)//{url}", "app_link_invalid"},
		{"\njavascript:alert(1)//{url}", "app_link_invalid"},
		{"java\nscript:alert(1)//{url}", "app_link_invalid"},
		{"java\tscript:alert(1)//{url}", "app_link_invalid"},
		{"java\x00script:alert(1)//{url}", "app_link_invalid"},
		{"myapp://add/{url} ", "app_link_invalid"},
		{"myapp://add/{url}\x7f", "app_link_invalid"},
		{"myapp://add/{ url }", "app_link_invalid"},
		// Letters that look like the Latin ones, invisible and bidirectional marks.
		{"jаvascript:alert(1)//{url}", "app_link_invalid"},          // a Cyrillic а
		{"ｊavascript:alert(1)//{url}", "app_link_invalid"},          // a full-width j
		{"javascript\uff1aalert(1)//{url}", "app_link_invalid"},     // a full-width colon
		{"javascript\ua789alert(1)//{url}", "app_link_invalid"},     // a modifier letter colon
		{"\ufeffmyapp://add/{url}", "app_link_invalid"},             // a byte order mark
		{"myapp://add/\u200b{url}", "app_link_invalid"},             // a zero-width space
		{"myapp://add/{url}\u202e", "app_link_invalid"},             // right-to-left override
		{"myapp://add/{url}\u00a0", "app_link_invalid"},             // a no-break space
		{"myapp://add/\xff{url}", "app_link_invalid"},               // not UTF-8
		{"myapp://add/{url}\uff5bname\uff5d", "app_link_invalid"},   // full-width braces
		{"myapp://add/\u2024{url}", "app_link_invalid"},             // a one-dot leader
		{"myapp://add/{url}/" + "\u0435xample", "app_link_invalid"}, // a Cyrillic е
		{long(501), "app_link_invalid"},

		// The subscription's address must be in it, and nothing but the placeholders in braces.
		{"myapp://add/", "app_link_url"},
		{"myapp://add/{name}", "app_link_url"},
		{"myapp://add/{URL}", "app_link_url"},
		{"myapp://add/{Url_Enc}", "app_link_url"},
		{"myapp://add/%7Burl%7D", "app_link_url"},
		{"myapp://add/%7burl_enc%7d", "app_link_url"},
		{"myapp://add/{url}{token}", "app_link_invalid"},
		{"myapp://add/{url}&name={Name}", "app_link_invalid"},
		{"myapp://add/{url}{", "app_link_invalid"},
		{"myapp://add/{url}}", "app_link_invalid"},
		{"myapp://add/{{url}}", "app_link_invalid"},
		{"myapp://add/{url_enc", "app_link_url"},
		{"myapp://add/{url}{url_enc", "app_link_invalid"},
	} {
		if got := AppLinkProblem(c.link); got != c.want {
			t.Errorf("AppLinkProblem(%q) = %q, want %q", c.link, got, c.want)
		}
	}
}
