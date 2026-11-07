package urlrequest

import (
	"encoding/base64"
	"net/url"
	"reflect"
	"testing"
)

func TestProtocolPreservesUnicodeAndQuery(t *testing.T) {
	target := "https://example.com/日本?q=a&b=two#section"
	for _, raw := range []string{"finicky://open?url=" + url.QueryEscape(target) + "&prompt=true", "finicky://open/" + base64.StdEncoding.EncodeToString([]byte(target))} {
		r, err := Parse(raw)
		if err != nil || r.URL != target {
			t.Fatalf("%q: %#v %v", raw, r, err)
		}
	}
}

func TestSourcePrivacyIsExplicitAndIndependentOfDestination(t *testing.T) {
	for _, test := range []struct {
		query, want string
	}{
		{"", "unknown"},
		{"&sourcePrivacy=public", "public"},
		{"&sourcePrivacy=private", "private"},
		{"&sourcePrivacy=unknown", "unknown"},
		{"&sourcePrivacy=PUBLIC", "unknown"},
		{"&sourcePrivacy=false", "unknown"},
		{"&sourcePrivacy=", "unknown"},
		{"&sourcePrivacy=public&sourcePrivacy=private", "unknown"},
		{"&sourcePrivacy=public&sourcePrivacy=public", "unknown"},
		{"&sourcePrivacy=public&sourcePrivacy=%zz", "unknown"},
	} {
		for _, action := range []string{"open", "copy", "share"} {
			raw := "finicky-plus://" + action + "?url=" + url.QueryEscape("https://example.com/日本?sourcePrivacy=public&x=1#fragment") + "&private=true" + test.query
			r, err := Parse(raw)
			if err != nil || r.SourcePrivacy != test.want || !r.Private || r.Action != action {
				t.Errorf("Parse(%q) = %#v, %v; want privacy %q and private destination", raw, r, err, test.want)
			}
		}
	}
}

func TestDirectAndLegacyURLsHaveUnknownSourcePrivacy(t *testing.T) {
	for _, raw := range []string{
		"https://example.com/?sourcePrivacy=public",
		"file:///tmp/page.html",
		"finicky://open/" + base64.RawURLEncoding.EncodeToString([]byte("https://example.com")) + "?sourcePrivacy=public",
	} {
		r, err := Parse(raw)
		if err != nil || r.SourcePrivacy != "unknown" {
			t.Errorf("Parse(%q) = %#v, %v; want unknown source privacy", raw, r, err)
		}
	}
}
func TestRejectInvalidRequests(t *testing.T) {
	for _, raw := range []string{"javascript:alert(1)", "finicky://open?url=finicky://open", "finicky://set-default", "finicky://open?url=https://example.com&profileDirectory=../Default", "finicky-plus://open?url=file:///tmp/code.sh&browser=Terminal", "finicky://open/" + base64.StdEncoding.EncodeToString([]byte("finicky://set-default?browser=Safari"))} {
		if _, err := Parse(raw); err == nil {
			t.Errorf("accepted %q", raw)
		}
	}
}
func TestExtractMultipleLinks(t *testing.T) {
	links := Extract("Visit https://a.example/a and <https://b.example/b>. https://a.example/a")
	if len(links) != 2 {
		t.Fatalf("%v", links)
	}
}

func TestExtractPreservesBalancedURLDelimiters(t *testing.T) {
	wiki := "https://en.wikipedia.org/wiki/Function_(mathematics)"
	for _, test := range []struct {
		name, text, want string
	}{
		{"bare parentheses", wiki, wiki},
		{"prose parentheses", "See (" + wiki + ").", wiki},
		{"angle wrapper", "See <" + wiki + ">.", wiki},
		{"quoted wrapper", "See \"" + wiki + "\";", wiki},
		{"nested parentheses", "See (https://example.com/f_(a_(b))).", "https://example.com/f_(a_(b))"},
		{"query brackets", "See [https://example.com/?tags=[work]].", "https://example.com/?tags=[work]"},
		{"query braces", "See {https://example.com/?q={work}}.", "https://example.com/?q={work}"},
		{"IPv6 host", "http://[::1]", "http://[::1]"},
		{"wrapped IPv6 host", "See [http://[::1]].", "http://[::1]"},
		{"encoded delimiter", "See (https://example.com/path%29).", "https://example.com/path%29"},
		{"ordinary prose punctuation", "See https://example.com/path,", "https://example.com/path"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := Extract(test.text); !reflect.DeepEqual(got, []string{test.want}) {
				t.Fatalf("Extract(%q) = %q, want %q", test.text, got, test.want)
			}
		})
	}
}

func TestExtractDeduplicatesAfterRemovingProseDelimiters(t *testing.T) {
	link := "https://example.com/path_(one)"
	text := "Read (" + link + "), then <" + link + ">; and https://example.com/?q=[two]."
	want := []string{link, "https://example.com/?q=[two]"}
	if got := Extract(text); !reflect.DeepEqual(got, want) {
		t.Fatalf("Extract(%q) = %q, want %q", text, got, want)
	}
}

func TestDirectURLParsingPreservesExactPunctuation(t *testing.T) {
	for _, raw := range []string{"https://example.com/path_(a)", "https://example.com/path).", "https://example.com/?q=[work]&encoded=%29", "http://[::1]"} {
		request, err := Parse(raw)
		if err != nil || request.URL != raw {
			t.Fatalf("Parse(%q) = %#v, %v", raw, request, err)
		}
	}
}
