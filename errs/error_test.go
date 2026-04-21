package errs

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
)

func TestHTTPErrorRedactsSensitiveMeta(t *testing.T) {
	type credentials struct {
		Token string `json:"token" pulse:"sensitive"`
		Name  string `json:"name"`
	}

	rec := httptest.NewRecorder()
	HTTPError(rec, B().Code(InvalidArgument).Msg("bad input").Meta("credentials", credentials{
		Token: "secret",
		Name:  "visible",
	}).Err())

	var got map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	meta := got["meta"].(map[string]any)
	creds := meta["credentials"].(map[string]any)
	if creds["token"] != "[redacted]" {
		t.Fatalf("token = %#v, want %q", creds["token"], "[redacted]")
	}
	if creds["name"] != "visible" {
		t.Fatalf("name = %#v, want %q", creds["name"], "visible")
	}
}
