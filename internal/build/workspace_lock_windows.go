//go:build windows

package build

func lockWorkspace(string) (func(), error) {
	return func() {}, nil
}
