package urlrequest

import (
	"encoding/base64"
	"net/url"
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
