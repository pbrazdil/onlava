package stdlog

import (
	"bytes"
	"testing"
)

func TestFilterWriterSuppressesIdleHTTPChannelNoise(t *testing.T) {
	var out bytes.Buffer
	w := &filterWriter{out: &out}

	n, err := w.Write([]byte(`2026/04/20 13:26:37 INFO Unsolicited response received on idle HTTP channel starting with "HTTP/1.1 400 Bad Request\r\n\r\n"; err=<nil>`))
	if err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if n == 0 {
		t.Fatal("Write() = 0, want consumed bytes")
	}
	if out.Len() != 0 {
		t.Fatalf("output = %q, want empty", out.String())
	}
}

func TestFilterWriterPassesOtherLogsThrough(t *testing.T) {
	var out bytes.Buffer
	w := &filterWriter{out: &out}

	input := []byte("2026/04/20 13:26:37 some other log\n")
	n, err := w.Write(input)
	if err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if n != len(input) {
		t.Fatalf("Write() = %d, want %d", n, len(input))
	}
	if out.String() != string(input) {
		t.Fatalf("output = %q, want %q", out.String(), string(input))
	}
}
