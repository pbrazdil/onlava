package runtime

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestExecuteTypedEndpointUsesMock(t *testing.T) {
	restore := replaceGlobalRegistryForTest()
	defer restore()
	ClearMocks()
	defer ClearMocks()

	type response struct {
		Message string `json:"message"`
	}

	ep := &Endpoint{
		Service:      "svc",
		Name:         "Hello",
		Methods:      []string{http.MethodPost},
		PayloadType:  TypeOf[*response](),
		ResponseType: TypeOf[*response](),
		Invoke: func(context.Context, []any, any) (any, error) {
			return &response{Message: "real"}, nil
		},
	}
	ref := func(context.Context, *response) (*response, error) { return nil, nil }
	RegisterEndpointFunc(ref, "svc", "Hello")
	restoreMock, err := SetEndpointMock(ref, func(ctx context.Context, req *response) (*response, error) {
		return &response{Message: "mock:" + req.Message}, nil
	})
	if err != nil {
		t.Fatalf("SetEndpointMock() error = %v", err)
	}
	defer restoreMock()

	got, _, _, err := executeTypedEndpoint(ep, context.Background(), nil, &response{Message: "ok"})
	if err != nil {
		t.Fatalf("executeTypedEndpoint() error = %v", err)
	}
	if got.(*response).Message != "mock:ok" {
		t.Fatalf("mocked response = %q, want %q", got.(*response).Message, "mock:ok")
	}
}

func TestLookupServiceMockUsesFactory(t *testing.T) {
	ClearMocks()
	defer ClearMocks()

	type service struct {
		Name string
	}

	restoreMock, err := SetServiceMock(func() (*service, error) {
		return &service{Name: "mock"}, nil
	})
	if err != nil {
		t.Fatalf("SetServiceMock() error = %v", err)
	}
	defer restoreMock()

	got, ok, err := LookupServiceMock(TypeOf[*service]())
	if err != nil {
		t.Fatalf("LookupServiceMock() error = %v", err)
	}
	if !ok {
		t.Fatal("LookupServiceMock() ok = false, want true")
	}
	if got.(*service).Name != "mock" {
		t.Fatalf("LookupServiceMock() value = %#v", got)
	}
}

func TestExecuteRawEndpointUsesMock(t *testing.T) {
	restore := replaceGlobalRegistryForTest()
	defer restore()
	ClearMocks()
	defer ClearMocks()

	ep := &Endpoint{
		Service: "svc",
		Name:    "Raw",
		Raw:     true,
		Methods: []string{"*"},
		RawHandler: func(w http.ResponseWriter, req *http.Request) {
			w.WriteHeader(http.StatusTeapot)
		},
	}
	ref := func(http.ResponseWriter, *http.Request) {}
	RegisterEndpointFunc(ref, "svc", "Raw")
	restoreMock, err := SetEndpointMock(ref, func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte("mock"))
	})
	if err != nil {
		t.Fatalf("SetEndpointMock() error = %v", err)
	}
	defer restoreMock()

	status, _, body, err := executeRawEndpoint(ep, httptest.NewRequest(http.MethodGet, "/raw", nil))
	if err != nil {
		t.Fatalf("executeRawEndpoint() error = %v", err)
	}
	if status != http.StatusAccepted {
		t.Fatalf("status = %d, want %d", status, http.StatusAccepted)
	}
	if string(body) != "mock" {
		t.Fatalf("body = %q, want %q", string(body), "mock")
	}
}
