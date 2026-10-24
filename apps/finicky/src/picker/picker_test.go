package picker

import (
	"finicky/browser"
	"testing"
)

func TestNormalizePreservesRequestAndDisambiguatesShortcuts(t *testing.T) {
	request := Request{URL: "https://example.com", Destinations: []Destination{
		{Name: "Work", Shortcut: " W "},
		{Name: "Another work", Shortcut: "w"},
		{Browser: browser.BrowserConfig{Name: "Firefox"}, Shortcut: "2"},
		{Name: "Personal", Shortcut: "p"},
	}}
	got := normalizedRequest(request)
	if got.Destinations[0].Shortcut != "w" || got.Destinations[1].Shortcut != "" || got.Destinations[2].Shortcut != "" || got.Destinations[3].Shortcut != "p" {
		t.Fatalf("unexpected shortcuts: %+v", got.Destinations)
	}
	if got.Destinations[2].Name != "Firefox" {
		t.Fatal("missing browser name fallback")
	}
	if request.Destinations[0].Shortcut != " W " || request.Destinations[2].Name != "" {
		t.Fatal("normalization mutated the caller's request")
	}
}

func TestCompletionOnlyDeliversOnceAndCloses(t *testing.T) {
	id := nextID.Add(1)
	channel := make(chan Selection, 1)
	results.Lock()
	results.pending[id] = channel
	results.Unlock()
	complete(id, Selection{Index: 12, Remember: true})
	complete(id, Selection{Index: -1, Cancelled: true})
	got, open := <-channel
	if !open || got.Index != 12 || !got.Remember || got.Cancelled {
		t.Fatalf("unexpected result: %+v", got)
	}
	if _, open := <-channel; open {
		t.Fatal("result channel must close after one selection")
	}
}
