package stdlog

import (
	"bytes"
	"io"
	"log"
	"sync"
)

var ignoredSubstrings = [][]byte{
	[]byte(`Unsolicited response received on idle HTTP channel`),
}

type filterWriter struct {
	out io.Writer
	mu  sync.Mutex
}

func Install(out io.Writer) {
	if out == nil {
		out = io.Discard
	}
	log.SetOutput(&filterWriter{out: out})
}

func (w *filterWriter) Write(p []byte) (int, error) {
	for _, ignored := range ignoredSubstrings {
		if bytes.Contains(p, ignored) {
			return len(p), nil
		}
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.out.Write(p)
}
