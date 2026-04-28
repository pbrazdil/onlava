package wire

import (
	"bytes"
	"testing"
)

func TestRequestFrameRoundTrip(t *testing.T) {
	payload := []byte(`{"message":"hello"}`)
	pathParams := []byte(`{"id":"42"}`)

	data := EncodeRequestFrame("schema-123", pathParams, payload)
	got, ok, err := DecodeRequestFrame(data)
	if err != nil {
		t.Fatalf("DecodeRequestFrame() error = %v", err)
	}
	if !ok {
		t.Fatalf("DecodeRequestFrame() ok = false")
	}
	if got.SchemaHash != "schema-123" {
		t.Fatalf("SchemaHash = %q", got.SchemaHash)
	}
	if !bytes.Equal(got.PathParamsJSON, pathParams) {
		t.Fatalf("PathParamsJSON = %q", got.PathParamsJSON)
	}
	if !bytes.Equal(got.PayloadJSON, payload) {
		t.Fatalf("PayloadJSON = %q", got.PayloadJSON)
	}
}

func TestResponseFrameRoundTrip(t *testing.T) {
	payload := []byte(`{"code":"invalid_argument","message":"bad"}`)

	data := EncodeResponseFrame(400, true, payload)
	got, ok, err := DecodeResponseFrame(data)
	if err != nil {
		t.Fatalf("DecodeResponseFrame() error = %v", err)
	}
	if !ok {
		t.Fatalf("DecodeResponseFrame() ok = false")
	}
	if got.Status != 400 {
		t.Fatalf("Status = %d", got.Status)
	}
	if !got.Error {
		t.Fatalf("Error = false")
	}
	if !bytes.Equal(got.PayloadJSON, payload) {
		t.Fatalf("PayloadJSON = %q", got.PayloadJSON)
	}
}
