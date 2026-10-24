package logger

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
)

func TestFileLoggingToggleStopsWritesAndReusesFile(t *testing.T) {
	w := &logWriter{}
	_, _ = w.Write([]byte("before\n"))
	if err := w.setFile(true, t.TempDir()); err != nil {
		t.Fatal(err)
	}
	file := w.file
	path := file.Name()
	if err := w.setFile(true, t.TempDir()); err != nil || w.file != file {
		t.Fatal("repeated enable opened another file")
	}
	_, _ = w.Write([]byte("enabled\n"))
	if err := w.setFile(false, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := file.Write([]byte("should fail")); err == nil {
		t.Fatal("disabled file remained open")
	}
	_, _ = w.Write([]byte("after\n"))
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "before\nenabled\n" {
		t.Fatalf("disk logs=%q error=%v", data, err)
	}
	if err := w.setFile(false, ""); err != nil {
		t.Fatal("repeated disable failed")
	}
}

func TestConcurrentLoggingIsBounded(t *testing.T) {
	w := &logWriter{}
	var wg sync.WaitGroup
	for worker := range 8 {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for i := range 2000 {
				_, _ = w.Write([]byte(fmt.Sprintf("%d/%d %s\n", id, i, strings.Repeat("x", 100))))
			}
		}(worker)
	}
	wg.Wait()
	if w.buffer.Len() > maxMemoryLog {
		t.Fatal("memory log exceeded cap")
	}
}
