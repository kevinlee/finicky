package browseraccess

import (
	"errors"
	"sync"
	"testing"
)

func TestCompletionIsSingleDeliveryEvenWithConcurrentCallbacks(t *testing.T) {
	id := nextRequestID.Add(1)
	result := make(chan error, 1)
	pending.Lock()
	pending.requests[id] = result
	pending.Unlock()
	want := errors.New("selection cancelled")
	var group sync.WaitGroup
	for i := 0; i < 10; i++ {
		group.Add(1)
		go func() {
			defer group.Done()
			complete(id, want)
		}()
	}
	group.Wait()
	if got, open := <-result; !open || got != want {
		t.Fatalf("unexpected completion: %v, open=%v", got, open)
	}
	if _, open := <-result; open {
		t.Fatal("completion channel must close")
	}
}

func TestSuccessfulCompletionDeliversNil(t *testing.T) {
	id := nextRequestID.Add(1)
	result := make(chan error, 1)
	pending.Lock()
	pending.requests[id] = result
	pending.Unlock()
	complete(id, nil)
	if err, open := <-result; !open || err != nil {
		t.Fatalf("success was not delivered: %v, open=%v", err, open)
	}
}
