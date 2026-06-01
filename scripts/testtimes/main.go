package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"syscall"
	"time"
)

type goTestEvent struct {
	Action  string  `json:"Action"`
	Package string  `json:"Package"`
	Test    string  `json:"Test"`
	Elapsed float64 `json:"Elapsed"`
}

type testTiming struct {
	Seconds float64
	Status  string
	Package string
	Test    string
}

func main() {
	exitCode, err := run(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		if exitCode == 0 {
			exitCode = 1
		}
	}
	os.Exit(exitCode)
}

func run(args []string) (int, error) {
	if len(args) > 0 && args[0] == "--" {
		args = args[1:]
	}

	goArgs := []string{"test", "-count=1", "-json"}
	if len(args) == 0 {
		goArgs = append(goArgs, "./...")
	} else {
		goArgs = append(goArgs, args...)
	}

	cmd := exec.Command("go", goArgs...)
	cmd.Stderr = os.Stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return 1, err
	}
	if err := cmd.Start(); err != nil {
		return 1, err
	}
	started := time.Now()

	var rows []testTiming
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	for scanner.Scan() {
		var event goTestEvent
		if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
			continue
		}
		if event.Test == "" {
			continue
		}
		switch event.Action {
		case "pass", "fail", "skip":
			rows = append(rows, testTiming{
				Seconds: event.Elapsed,
				Status:  event.Action,
				Package: event.Package,
				Test:    event.Test,
			})
		}
	}
	scanErr := scanner.Err()
	waitErr := cmd.Wait()

	printTable(rows, time.Since(started))

	exitCode := 0
	if waitErr != nil {
		exitCode = commandExitCode(waitErr)
	}
	if scanErr != nil {
		return exitCode, scanErr
	}
	if waitErr != nil {
		return exitCode, waitErr
	}
	return 0, nil
}

func printTable(rows []testTiming, actualElapsed time.Duration) {
	var totalSeconds float64
	for _, row := range rows {
		totalSeconds += row.Seconds
	}

	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].Seconds != rows[j].Seconds {
			return rows[i].Seconds < rows[j].Seconds
		}
		if rows[i].Package != rows[j].Package {
			return rows[i].Package < rows[j].Package
		}
		return rows[i].Test < rows[j].Test
	})

	fmt.Printf("%8s  %-7s  %-60s  %s\n", "seconds", "status", "package", "test")
	fmt.Printf("%8s  %-7s  %-60s  %s\n", "-------", "------", "-------", "----")
	for _, row := range rows {
		fmt.Printf("%8.2f  %-7s  %-60s  %s\n", row.Seconds, row.Status, row.Package, row.Test)
	}
	fmt.Printf("\n%d tests\n", len(rows))
	fmt.Printf("total test time: %.2fs\n", totalSeconds)
	fmt.Printf("total actual time: %.2fs\n", actualElapsed.Seconds())
}

func commandExitCode(err error) int {
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
			return status.ExitStatus()
		}
	}
	return 1
}
