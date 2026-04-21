## Bottom line

The most likely reason Ctrl+C is still unreliable is **not that both supervisor and child have signal handlers in itself**. The real issue is that after `Setpgid: true`, the compiled app is no longer in the terminal’s foreground process group. Ctrl+C reaches the foreground group, which contains the supervisor and, in the `go run` case, the Go tool wrapper. The detached app only exits if the supervisor reliably forwards shutdown to its process group.

That model is valid, but the current shutdown path has three deterministic-failure gaps:

1. **The supervisor keeps intercepting Ctrl+C until `defer stop()` runs.**
   `signal.NotifyContext` changes future interrupt behavior until its `stop` function is called; future interrupts will not perform the default exit behavior until `stop()` is called. Your `watch.go` and `runtime/app.go` both defer `stop()`, so a second Ctrl+C during a stuck cleanup is still swallowed. Call `stop()` immediately after the first signal is observed. ([Go Packages][1])

2. **`devSupervisor.Close()` stops in-process services before it stops the app child.**
   Current order is roughly: proxy → dashboard → DB Studio → DB Studio UI → app. If Caddy `Stop()`, DB Studio shutdown, dashboard close, or a pipe wait stalls, the app process group may never receive SIGINT. For reliability, external process groups should be signaled first, listeners closed second, forced kills last.

3. **DB Studio and other `CommandContext` subprocesses are not consistently killed as process trees.**
   `exec.CommandContext` kills only the direct process by default; with `Setpgid`, you still need a custom cancel/cleanup path that sends the signal to the process group. This matters for `bun`, `bunx`, `drizzle-kit`, and any grandchildren they spawn. Go’s `exec.Cmd` docs also note that pipe-copying and `WaitDelay` can affect when `Wait` returns. ([Go Packages][2])

The child’s own signal handling should stay, but in supervised mode it should be treated as a handler for **supervisor-forwarded SIGINT/SIGTERM**, not for terminal Ctrl+C.

---

## Recommended subprocess topology

### Installed binary: `pulse run`

Use this model:

```text
terminal foreground process group
└── pulse supervisor
    ├── in-process dashboard HTTP server
    ├── in-process embedded Caddy proxy, if enabled
    ├── in-process DB Studio UI, if enabled
    ├── child process group: compiled app
    │   └── any app grandchildren
    └── child process group: DB Studio/bun/bunx, if enabled
        └── any DB Studio grandchildren
```

The supervisor owns terminal signals. The app child is in its own process group. On Ctrl+C:

```text
SIGINT to terminal foreground pgrp
→ supervisor receives SIGINT
→ supervisor immediately unregisters signal interception
→ supervisor cancels root context
→ supervisor sends SIGINT to child process groups
→ supervisor closes listeners
→ supervisor waits bounded grace period
→ supervisor sends SIGKILL to unreaped child process groups
→ process exits
```

### `go run ./cmd/pulse run ...`

Use the same runtime model:

```text
terminal foreground process group
├── go tool wrapper
└── compiled pulse supervisor
    ├── in-process dashboard/Caddy/etc.
    └── detached child process groups
```

Do **not** rely on the Go tool wrapper to clean anything. The compiled `pulse` process must handle Ctrl+C itself. Parent monitors are only a last-resort safety net for wrapper death or crash.

---

## Should the child inherit stdin?

I would remove this:

```go
cmd.Stdin = os.Stdin
```

For the supervised app, set stdin to nil or explicitly to `/dev/null`. In Go, a nil `Cmd.Stdin` makes the process read from `os.DevNull`. ([Go Packages][2])

Keeping `stdin = os.Stdin` is not the main signal-delivery bug, but it is the wrong ownership boundary. A background process group sharing the terminal’s stdin can receive terminal-driver surprises, accidental reads, EOF behavior, or `SIGTTIN`-style issues depending on platform and terminal state. The supervisor should own terminal input and signals.

If app stdin is needed later, make it an explicit mode such as `--forward-stdin`, then implement it as a supervised stream with cancellation and documented semantics. Do not make it the default.

---

## Concrete reliability changes

### 1. Stop intercepting Ctrl+C after the first signal

In `cmd/pulse/watch.go`, avoid `defer stop()` as the only unregister path. Use a goroutine that unregisters immediately after first signal:

```go
sigCtx, stopSignals := signal.NotifyContext(
	context.Background(),
	os.Interrupt,
	syscall.SIGTERM,
)

ctx, cancel := context.WithCancel(sigCtx)

go func() {
	<-sigCtx.Done()

	// Restore default signal behavior immediately.
	// A second Ctrl+C should hard-exit instead of being swallowed.
	stopSignals()

	cancel()
}()

defer func() {
	stopSignals()
	cancel()
}()
```

In `runtime/app.go`, do the same:

```go
sigCtx, stopSignals := signal.NotifyContext(
	context.Background(),
	os.Interrupt,
	syscall.SIGTERM,
)
defer stopSignals()

go func() {
	<-sigCtx.Done()

	// In supervised mode this signal normally comes from the supervisor.
	// In standalone mode it comes from the terminal.
	stopSignals()

	cancelRun()
}()
```

This is especially important because your shutdown path includes Caddy, HTTP servers, pipes, DB Studio subprocesses, and possibly app-level shutdown hooks. A second Ctrl+C must become an escape hatch.

---

### 2. Signal process groups before closing in-process services

Change `devSupervisor.Close()` from “close services, then stop app” to a two-phase shutdown.

Current order is effectively:

```text
proxy.Close()
dashboard.Close()
dbStudio.Close()
dbStudioUI.Close()
stopCurrent()
store.Close()
```

Recommended order:

```text
mark supervisor closing
send SIGINT to app process group
send SIGINT to DB Studio process group, if known
cancel supervisor context
close dashboard listener
close proxy listener
close DB Studio UI listener
wait bounded grace for app/dbstudio
send SIGKILL to remaining process groups
close store
```

Shape:

```go
func (s *devSupervisor) Close() error {
	var closeErr error

	s.closeOnce.Do(func() {
		// Phase 1: stop accepting new work and tell external trees to exit.
		if s.cancel != nil {
			s.cancel()
		}

		app := s.currentApp()
		dbs := s.currentDBStudio()

		if app != nil {
			app.interrupt()
		}
		if dbs != nil {
			dbs.interrupt()
		}

		// Phase 2: close in-process listeners concurrently and bounded.
		var wg sync.WaitGroup
		errs := make(chan error, 4)

		closeAsync := func(fn func() error) {
			wg.Add(1)
			go func() {
				defer wg.Done()
				if err := fn(); err != nil {
					errs <- err
				}
			}()
		}

		if s.dashboard != nil {
			closeAsync(s.dashboard.Close)
		}
		if s.proxy != nil {
			closeAsync(s.proxy.Close)
		}
		if s.dbStudioUI != nil {
			closeAsync(s.dbStudioUI.Close)
		}

		done := make(chan struct{})
		go func() {
			wg.Wait()
			close(done)
		}()

		select {
		case <-done:
		case <-time.After(2 * time.Second):
			errs <- fmt.Errorf("timed out closing in-process dev services")
		}

		// Phase 3: bounded graceful wait, then hard kill.
		if app != nil {
			if err := app.waitOrKill(5 * time.Second); err != nil {
				errs <- err
			}
		}
		if dbs != nil {
			if err := dbs.waitOrKill(5 * time.Second); err != nil {
				errs <- err
			}
		}

		if s.store != nil {
			if err := s.store.Close(); err != nil {
				errs <- err
			}
		}

		close(errs)
		closeErr = errors.Join(sliceErrs(errs)...)
	})

	return closeErr
}
```

The exact types will differ, but the key is the ordering: **interrupt external trees first, close listeners second, kill last**.

---

### 3. Split `runningApp.stop()` into interrupt and wait/kill

Today `runningApp.stop()` does everything inline. That makes it hard for the supervisor to broadcast SIGINT to multiple process trees before waiting on any one of them.

Refactor toward:

```go
func (a *runningApp) interrupt() error {
	if a == nil || a.cmd == nil || a.cmd.Process == nil {
		return nil
	}
	return signalProcessTree(a.cmd, syscall.SIGINT)
}

func (a *runningApp) kill() error {
	if a == nil || a.cmd == nil || a.cmd.Process == nil {
		return nil
	}
	return signalProcessTree(a.cmd, syscall.SIGKILL)
}

func (a *runningApp) waitOrKill(grace time.Duration) error {
	if a == nil {
		return nil
	}

	select {
	case err := <-a.done:
		return normalizeExitErr(err)

	case <-time.After(grace):
		_ = a.kill()

		select {
		case err := <-a.done:
			return normalizeExitErr(err)

		case <-time.After(1 * time.Second):
			return fmt.Errorf("app did not exit after SIGKILL")
		}
	}
}
```

Then the supervisor can call `interrupt()` for all children first, and only then wait.

---

### 4. Signal the actual process group, not `-cmd.Process.Pid`

Your current Unix helper assumes `pgid == pid`. That is true when `Setpgid: true` and `Pgid == 0`, but use `Getpgid` anyway. It is safer and makes future refactors less fragile.

Go’s `SysProcAttr.Setpgid` sets the child’s process group ID to `Pgid`, or to the child PID when `Pgid == 0`. ([Go][3])

Recommended helper:

```go
func signalProcessTree(cmd *exec.Cmd, sig syscall.Signal) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}

	pid := cmd.Process.Pid

	pgid, err := syscall.Getpgid(pid)
	if err == nil && pgid > 1 {
		if err := syscall.Kill(-pgid, sig); err == nil {
			return nil
		}

		// ESRCH means already gone.
		if errors.Is(err, syscall.ESRCH) {
			return nil
		}

		return err
	}

	// Fallback: direct child only.
	err = cmd.Process.Signal(sig)
	if errors.Is(err, os.ErrProcessDone) || errors.Is(err, syscall.ESRCH) {
		return nil
	}
	return err
}
```

On Unix, keep:

```go
cmd.SysProcAttr = &syscall.SysProcAttr{
	Setpgid: true,
}
```

Do **not** use `Foreground: true` for the app. That would hand the terminal to the app and make the supervisor no longer the owner of Ctrl+C semantics.

---

### 5. Remove app stdin inheritance

In `cmd/pulse/dev_supervisor.go`:

```diff
- cmd.Stdin = os.Stdin
+ // Keep the supervised app detached from terminal input.
+ // The supervisor owns Ctrl+C and forwards shutdown explicitly.
+ cmd.Stdin = nil
```

Or simply omit `cmd.Stdin`.

---

### 6. Make all external commands process-group-aware

This applies to:

* compiled app
* DB Studio `bun`
* DB Studio `bunx drizzle-kit studio`
* `drizzle-kit pull`
* `bun install`
* dashboard UI install/build commands, if any external commands are used there

For commands currently using `exec.CommandContext`, do not rely on default cancellation. Default `CommandContext` uses `Process.Kill`; it does not kill grandchildren. ([Go Packages][2])

Use a helper like:

```go
func commandTreeContext(ctx context.Context, name string, args ...string) *exec.Cmd {
	cmd := exec.Command(name, args...)
	configureChildProcess(cmd)

	go func() {
		<-ctx.Done()

		if cmd.Process == nil {
			return
		}

		_ = signalProcessTree(cmd, syscall.SIGINT)

		timer := time.NewTimer(3 * time.Second)
		defer timer.Stop()

		select {
		case <-timer.C:
			_ = signalProcessTree(cmd, syscall.SIGKILL)
		}
	}()

	return cmd
}
```

A slightly cleaner implementation is to wrap `Start`/`Wait` so the cancellation goroutine starts only after `cmd.Start()` succeeds. Also consider `cmd.WaitDelay` where pipe or grandchild FD inheritance can keep `Wait` blocked; Go exposes `WaitDelay` specifically to bound waiting when the process exits but pipes remain open or when a context is canceled. ([Go Packages][2])

---

### 7. Replace the dashboard orphan heuristic with a run-state file

The existing dashboard startup logic uses `lsof`/`ps` and then kills processes whose command line looks like Pulse. That is useful as a last resort, but it is inherently heuristic.

Use a state file under a stable cache/run directory:

```json
{
  "supervisor_pid": 12345,
  "supervisor_pgid": 12345,
  "started_at": "2026-04-20T12:34:56Z",
  "app_root": "/Users/petrbrazdil/Repos/onlv",
  "dashboard_addr": "127.0.0.1:..."
}
```

On startup:

1. Read the state file.
2. Check whether the PID exists.
3. Verify process identity and start time if available.
4. Verify same app root and dashboard addr.
5. If stale, remove the file.
6. If alive and same app root, terminate its process group.
7. Fall back to `lsof` only when no trustworthy state file exists.

The old symptom “orphan dashboard listener on dashboard port” means the **supervisor process** survived or got wedged, because the dashboard is in-process. Treat dashboard-port cleanup as supervisor-run cleanup, not app cleanup.

---

### 8. Improve the parent monitor but keep it as backup only

The current PPID monitor is useful, but it is not deterministic. It polls every second, and PPID changes are a crash fallback, not a primary shutdown protocol.

Improve it by passing an exact supervisor PID:

```go
cmd.Env = append(cmd.Env,
	"PULSE_DEV_SUPERVISOR=1",
	"PULSE_DEV_SUPERVISOR_PID="+strconv.Itoa(os.Getpid()),
)
```

Then the child can monitor that PID directly. On Linux, `Pdeathsig` can be an additional belt-and-suspenders mechanism, but it should not be the primary design. Go’s Linux `SysProcAttr.Pdeathsig` docs note that the signal is tied to the creating thread’s death, not just clean process termination, so treat it as a platform-specific backup. ([Go][4])

---

## Embedded Caddy: reliability and performance assessment

The embedded Caddy proxy is probably not the direct cause of Ctrl+C failing to reach the child app, but it is a high-risk component in shutdown and startup because it is:

* in-process,
* process-global,
* listener-owning,
* certificate/trust-store capable,
* imported into the build graph.

Caddy’s `Load` runs a config, and `Stop` stops the current running config. Caddy also documents `TrapSignals` as invasive when the host program already captures signals; you are not calling it from the inspected code, which is good. ([Go Packages][5])

The local HTTPS behavior is worth changing. Caddy’s `local_certs` causes internal cert issuance, and Caddy docs state that the first use of a root key may try to install the root into system trust stores and may prompt for a password unless disabled via `skip_install_trust` / `"install_trust": false`. ([Caddy Web Server][6])

### Recommended Caddy changes

For local-dev reliability first:

1. **Make the local proxy explicitly opt-in.**
   In the inspected code, `startLocalProxy` effectively enables proxying by default because `DiscoverWorkspace(s.root, s.cfg.Name)` will usually produce a workspace label. I would change that to require one of:

   * `--proxy`
   * explicit config
   * `PULSE_LOCAL_PROXY=1`

2. **Default to `skip_install_trust` true.**
   Add a separate `pulse trust` or `pulse proxy trust` command for users who want system-trusted local HTTPS.

3. **Do not close Caddy before signaling child process groups.**
   Caddy should be part of phase-two listener cleanup, not the first thing in `Close()`.

For build/startup speed:

4. **Avoid the Caddyfile adapter.**
   You currently import the Caddyfile adapter and reverse proxy modules. Generate Caddy JSON directly. This does not remove all Caddy cost, but it trims adapter-related initialization and dependency surface.

5. **Consider replacing embedded Caddy with a smaller dev proxy.**
   For a local reverse proxy, `net/http/httputil.ReverseProxy` plus cached local certificates is far cheaper than embedding Caddy. The contrarian but likely high-payoff version is:

   * default: simple HTTP reverse proxy, no Caddy
   * optional: Caddy sidecar binary or build tag for full local HTTPS behavior
   * explicit trust command for certificates

6. **Move Caddy behind a build tag or separate command package.**
   Environment flags do not reduce Go compile cost if packages remain imported. The current unconditional imports mean the CLI and app runtime can pay for Caddy even when proxying is disabled.

---

## Highest-payoff performance wins

Ranked by likely payoff from the inspected source:

### 1. Split the runtime/package graph

This is the biggest structural win.

Several CLI-side packages appear to import `pulse.dev/runtime` just for small types or constants. That drags in `runtime/app.go`, which imports local proxy, DB Studio, pubsub, and other heavy runtime pieces. That in turn pulls in Caddy and NATS-related dependencies.

Create a leaf package, for example:

```text
pulse.dev/runtime/types
```

or

```text
pulse.dev/internal/runtimeapi
```

Move small shared types there:

```go
type Access ...
type ParamKind ...
```

Then:

* parser/model/codegen/clientgen import the leaf package
* `pulse.dev/runtime` re-exports aliases for compatibility
* generated app code can still import the full runtime where needed
* the CLI no longer compiles the world just to parse schemas

This is likely a larger win than toggling cgo.

---

### 2. Split standalone runtime conveniences from supervised runtime

`runtime/app.go` includes logic for local proxy and DB Studio when not launched by the supervisor. That is convenient, but it makes every app runtime import local-dev infrastructure.

I would split:

```text
runtime/
  app.go                 // core app lifecycle
  supervised.go          // supervisor reporting/parent monitor
  standalone_dev.go      // local proxy + dbstudio, behind build tag or separate package
```

Or:

```go
pulse.dev/runtime/devlocal
```

Then the supervised child app path does not need to compile Caddy or DB Studio at all.

---

### 3. Move pubsub/NATS behind a registration boundary

`runtime` imports `pulse.dev/pubsub`, and `pubsub` imports embedded NATS. Even if an app declares no topics, that cost is in the package graph.

Make NATS runtime startup conditional at the import graph level, not just runtime behavior. For example:

```text
pulse.dev/pubsub/runtime
pulse.dev/pubsub/natslocal
```

Generated code or feature registration can import the heavier package only when pubsub is used.

---

### 4. Fix DB Studio startup cost

`internal/dbstudio/dbstudio.go` currently removes and recreates the workspace, writes a package file, runs install, pulls schema, then starts Studio. When DB Studio is enabled, this is a large startup cost and a large shutdown-risk surface.

High-payoff changes:

* cache workspace by dependency-version hash
* pin versions instead of `"latest"`
* keep a lockfile
* avoid reinstalling when package metadata has not changed
* make every install/pull/studio command process-group-cancellable
* defer DB Studio startup until requested in the UI

---

### 5. cgo: measure, then probably default the CLI to pure Go

I found no `import "C"` in the repo source bundle. That does not prove no transitive cgo is used, but it makes cgo less likely to be the main issue.

Run:

```bash
go list -deps -f '{{if .CgoFiles}}{{.ImportPath}} {{.CgoFiles}}{{end}}' ./cmd/pulse
```

Then compare:

```bash
CGO_ENABLED=0 /usr/bin/time -l go build ./cmd/pulse
CGO_ENABLED=1 /usr/bin/time -l go build ./cmd/pulse
```

Also test:

```bash
CGO_ENABLED=0 go build -tags=netgo ./cmd/pulse
```

The Go docs state that `CGO_ENABLED=0` disables cgo files, and that the `netgo` build tag disables use of the native cgo resolver. On macOS, the `net` package may use the cgo resolver under common conditions when cgo is available. ([Go Packages][7])

My recommendation:

* build the `pulse` CLI with `CGO_ENABLED=0` by default if dependencies allow it
* keep child app cgo controlled by the app’s own build configuration
* use `-tags=netgo` for the CLI unless you discover a concrete resolver incompatibility

---

### 6. Replace polling watcher with fs events or reduce scan cost

`scanWatchedFiles` polls the tree every 250ms. In a large app repo, that is continuous idle CPU and filesystem pressure.

Immediate improvement:

* skip more generated/build/cache directories:

  * `.next`
  * `dist`
  * `build`
  * `tmp`
  * `.turbo`
  * `.cache`
  * `coverage`
* include `go.mod`, `go.sum`, `.env`, and config files if they affect rebuilds
* debounce by event batch, not by polling interval

Better improvement:

* use `fsnotify`
* keep a fallback polling mode for network filesystems

---

## Priority patch plan

### Reliability first

1. Call `stopSignals()` immediately after first signal in both supervisor and runtime.
2. Remove supervised app stdin inheritance.
3. Make `Close()` idempotent with `sync.Once`.
4. Signal app and DB Studio process groups before closing dashboard/Caddy/UI.
5. Refactor child stop into `interrupt()` and `waitOrKill()`.
6. Use `Getpgid` in `signalProcessTree`.
7. Make all external commands process-group-cancellable.
8. Add run-state file cleanup for dashboard-port ownership.
9. Keep parent monitors only as crash fallback.

### Startup/build speed second

1. Move shared runtime types into a leaf package.
2. Stop importing full runtime from parser/model/clientgen paths.
3. Split local proxy / DB Studio / standalone dev support out of the core runtime.
4. Put embedded Caddy behind explicit opt-in, build tag, or sidecar binary.
5. Replace Caddyfile adapter with direct JSON if Caddy remains embedded.
6. Cache DB Studio workspace and pin dependencies.
7. Test `CGO_ENABLED=0 -tags=netgo` for the CLI.
8. Replace 250ms tree polling with fsnotify/debounce.

---

## Tests I would add

### Ctrl+C propagation test

Spawn a fake app that:

* starts a listener
* spawns a grandchild
* ignores nothing
* exits gracefully on SIGINT

Then run supervisor, send SIGINT to supervisor, assert:

```text
supervisor exits
child exits
grandchild exits
listener port is free
```

### Stuck child test

Fake app ignores SIGINT. Assert:

```text
SIGINT sent
5s grace expires
SIGKILL sent to process group
listener port is free
```

### `go run` wrapper test

Spawn a wrapper process that launches supervisor and then dies. Assert parent monitor eventually cancels, but also verify normal Ctrl+C works without relying on the monitor.

### DB Studio tree test

Use a fake `bun` that spawns a sleeping grandchild and keeps file descriptors open. Cancel startup and assert both direct process and grandchild are gone.

### Second Ctrl+C test

Create a shutdown hook that blocks. Send SIGINT twice. Assert the second interrupt exits by default rather than being swallowed.

---

## My concrete answer to your specific questions

**Is the signal topology wrong because both supervisor and child consume terminal signals?**
Not exactly. The topology is acceptable if the child is detached and the supervisor is the signal owner. The bug is that the supervisor does not currently behave like a deterministic signal owner: it defers signal unregistration, closes services before interrupting external trees, and does not uniformly kill subprocess groups.

**Is the child inheriting stdin part of the problem?**
It is probably not the primary Ctrl+C cause, but it is the wrong default. Remove stdin inheritance. Supervised children should not share terminal input unless you explicitly implement terminal brokering.

**What subprocess model should work for both installed `pulse run` and `go run ./cmd/pulse run ...`?**
Supervisor in foreground; app/DB Studio in detached process groups; supervisor owns terminal signals; supervisor forwards SIGINT/SIGTERM to process groups; parent monitors are backup only.

**Highest-payoff performance gains?**
First: import graph surgery. Split runtime types and remove full runtime imports from CLI parse/model/codegen paths. Second: split Caddy/DB Studio/NATS out of core runtime and make local proxy opt-in. Third: cache DB Studio setup. Fourth: test `CGO_ENABLED=0 -tags=netgo`; cgo may help, but it is unlikely to beat the package-graph wins.

**What would I change first in this repo?**
I would patch signal unregistration, stdin, shutdown ordering, process-group cancellation, and DB Studio command cancellation before touching Caddy. Then I would make the local proxy explicit opt-in and split the runtime import graph.

---

## Source URLs

* Go `os/signal.NotifyContext`: [https://pkg.go.dev/os/signal#NotifyContext](https://pkg.go.dev/os/signal#NotifyContext)
* Go `os/exec.Cmd`: [https://pkg.go.dev/os/exec#Cmd](https://pkg.go.dev/os/exec#Cmd)
* Go `syscall.SysProcAttr` on Darwin: [https://pkg.go.dev/syscall?GOOS=darwin#SysProcAttr](https://pkg.go.dev/syscall?GOOS=darwin#SysProcAttr)
* Go Linux `SysProcAttr.Pdeathsig` source: [https://go.dev/src/syscall/exec_linux.go](https://go.dev/src/syscall/exec_linux.go)
* Caddy package docs: [https://pkg.go.dev/github.com/caddyserver/caddy/v2](https://pkg.go.dev/github.com/caddyserver/caddy/v2)
* Caddy Caddyfile global options: [https://caddyserver.com/docs/caddyfile/options](https://caddyserver.com/docs/caddyfile/options)
* Caddy Automatic HTTPS: [https://caddyserver.com/docs/automatic-https](https://caddyserver.com/docs/automatic-https)
* Go `net` resolver docs: [https://pkg.go.dev/net](https://pkg.go.dev/net)
* Go cgo docs: [https://pkg.go.dev/cmd/cgo](https://pkg.go.dev/cmd/cgo)

[1]: https://pkg.go.dev/os/signal "signal package - os/signal - Go Packages"
[2]: https://pkg.go.dev/os/exec "exec package - os/exec - Go Packages"
[3]: https://go.dev/pkg/syscall/?GOOS=darwin "syscall package - syscall - Go Packages"
[4]: https://go.dev/src/syscall/exec_linux.go " - The Go Programming Language"
[5]: https://pkg.go.dev/github.com/caddyserver/caddy/v2 "caddy package - github.com/caddyserver/caddy/v2 - Go Packages"
[6]: https://caddyserver.com/docs/caddyfile/options "Global options (Caddyfile) — Caddy Documentation"
[7]: https://pkg.go.dev/net "net package - net - Go Packages"
