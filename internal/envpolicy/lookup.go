package envpolicy

import "os"

func Get(name string) string {
	return os.Getenv(name)
}

func Lookup(name string) (string, bool) {
	return os.LookupEnv(name)
}

func Environ() []string {
	return os.Environ()
}

func Set(name, value string) error {
	return os.Setenv(name, value)
}

func Unset(name string) error {
	return os.Unsetenv(name)
}
