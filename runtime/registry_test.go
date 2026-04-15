package runtime

import (
	"errors"
	"sync"
	"testing"
	"time"

	"pulse.dev/runtime/shared"
)

func TestInitializeServicesRunsInParallel(t *testing.T) {
	restore := replaceGlobalRegistryForTest()
	defer restore()

	started := make(chan string, 2)
	release := make(chan struct{})
	var done sync.WaitGroup
	done.Add(1)
	errCh := make(chan error, 1)

	blockingInit := func(name string) func() error {
		return func() error {
			started <- name
			<-release
			return nil
		}
	}
	RegisterServiceInitializer("zeta", blockingInit("zeta"))
	RegisterServiceInitializer("alpha", blockingInit("alpha"))

	go func() {
		defer done.Done()
		errCh <- InitializeServices()
	}()

	seen := map[string]bool{}
	for len(seen) < 2 {
		select {
		case name := <-started:
			seen[name] = true
		case <-time.After(2 * time.Second):
			t.Fatalf("timed out waiting for service initializers to start; saw %v", seen)
		}
	}
	close(release)
	done.Wait()

	if err := <-errCh; err != nil {
		t.Fatalf("InitializeServices() error = %v", err)
	}
	if !seen["alpha"] || !seen["zeta"] {
		t.Fatalf("InitializeServices() started = %v, want both services", seen)
	}
}

func TestInitializeServicesPropagatesErrors(t *testing.T) {
	restore := replaceGlobalRegistryForTest()
	defer restore()

	RegisterServiceInitializer("service", func() error {
		return errors.New("boom")
	})

	err := InitializeServices()
	if err == nil || err.Error() != "initialize service service: boom" {
		t.Fatalf("InitializeServices() error = %v, want initialize service service: boom", err)
	}
}

func replaceGlobalRegistryForTest() func() {
	prev := global
	global = &registry{
		endpoints:           make(map[string]*Endpoint),
		middlewares:         make(map[string]*Middleware),
		cronJobs:            make(map[string]*CronJob),
		serviceInitializers: make(map[string]func() error),
		meta: shared.AppMetadata{
			Environment: shared.Environment{
				Name:  "local",
				Type:  shared.EnvDevelopment,
				Cloud: shared.CloudLocal,
			},
		},
	}
	return func() {
		global = prev
	}
}
