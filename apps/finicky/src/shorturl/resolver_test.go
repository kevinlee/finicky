package shorturl

import (
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func TestShortenerDomainBoundary(t *testing.T) {
	for _, test := range []struct {
		host, domain string
		want         bool
	}{
		{"bit.ly", "bit.ly", true}, {"sub.bit.ly", "bit.ly", true}, {"evilbit.ly", "bit.ly", false}, {"bit.ly.evil.test", "bit.ly", false}, {"example.com", "", false},
	} {
		if got := matchesDomain(test.host, test.domain); got != test.want {
			t.Errorf("%s / %s = %v", test.host, test.domain, got)
		}
	}
	got, err := ResolveURL("https://evilbit.ly/test")
	if err != nil || got != "https://evilbit.ly/test" {
		t.Fatalf("non-shortener triggered a request: %s %v", got, err)
	}
}

type trackedBody struct {
	io.Reader
	closed bool
}

func (b *trackedBody) Close() error { b.closed = true; return nil }

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestResponsesAlwaysClose(t *testing.T) {
	for _, headStatus := range []int{200, 405} {
		t.Run(http.StatusText(headStatus), func(t *testing.T) {
			var bodies []*trackedBody
			previous := http.DefaultTransport
			t.Cleanup(func() { http.DefaultTransport = previous })
			http.DefaultTransport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
				body := &trackedBody{Reader: strings.NewReader("body")}
				bodies = append(bodies, body)
				status := 200
				if r.Method == http.MethodHead {
					status = headStatus
				}
				return &http.Response{StatusCode: status, Body: body, Request: r, Header: http.Header{}}, nil
			})
			got, err := ResolveURLWithDomains("https://short.example/link", []string{"short.example"})
			if err != nil || got != "https://short.example/link" {
				t.Fatalf("got=%s err=%v", got, err)
			}
			for _, b := range bodies {
				if !b.closed {
					t.Fatal("response body leaked")
				}
			}
			want := 1
			if headStatus == 405 {
				want = 2
			}
			if len(bodies) != want {
				t.Fatalf("requests=%d want=%d", len(bodies), want)
			}
		})
	}
}

func TestHostnameIgnoresPortAndNormalizesCase(t *testing.T) {
	u, _ := url.Parse("https://BIT.LY:443/path")
	if !matchesDomain(strings.ToLower(u.Hostname()), "bit.ly") {
		t.Fatal("hostname with port not recognized")
	}
}
