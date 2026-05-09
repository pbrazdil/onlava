package data

import (
	"errors"
	"testing"
)

func TestCodeOfClassifiesPublicDataErrors(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want ErrorCode
	}{
		{name: "nil", want: ""},
		{name: "wrapped", err: wrapError("QueryRecords", errors.New("cursor sort shape does not match query sort")), want: ErrorInvalidCursor},
		{name: "field", err: errors.New(`selected field "missing" does not exist on object company`), want: ErrorFieldNotFound},
		{name: "filter", err: errors.New("operator contains is not valid for field arr of type numeric"), want: ErrorInvalidFilter},
		{name: "drift", err: errors.New("physical schema drift was detected"), want: ErrorSchemaDrift},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CodeOf(tt.err); got != tt.want {
				t.Fatalf("CodeOf() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestErrorUnwrap(t *testing.T) {
	inner := errors.New("inner")
	err := wrapError("CreateObject", inner)
	if !errors.Is(err, inner) {
		t.Fatalf("wrapped error does not unwrap to inner: %v", err)
	}
	var dataErr *Error
	if !errors.As(err, &dataErr) || dataErr.Op != "CreateObject" {
		t.Fatalf("wrapped error = %#v", err)
	}
}
