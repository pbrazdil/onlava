package et

import "scenery.sh/runtime"

type helperT interface {
	Helper()
	Cleanup(func())
}

func MockEndpoint(t helperT, ref, mock any) {
	if t != nil {
		t.Helper()
	}
	restore, err := runtime.SetEndpointMock(ref, mock)
	if err != nil {
		panic(err)
	}
	if t != nil {
		t.Cleanup(restore)
	}
}

func MockService(t helperT, mock any) {
	if t != nil {
		t.Helper()
	}
	restore, err := runtime.SetServiceMock(mock)
	if err != nil {
		panic(err)
	}
	if t != nil {
		t.Cleanup(restore)
	}
}

func ClearMocks() {
	runtime.ClearMocks()
}
