//go:build windows

package neonselfhost

func lockBackendState(string) (func(), error) {
	return func() {}, nil
}
