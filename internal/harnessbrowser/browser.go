package harnessbrowser

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/cdproto/network"
	cdpRuntime "github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
)

type RouteSpec struct {
	Name    string
	Path    string
	Markers []string
}

type Result struct {
	Routes          []Route
	ConsoleErrors   []ConsoleMessage
	NetworkFailures []NetworkFailure
	Artifacts       []Artifact
}

type Route struct {
	Name            string
	URL             string
	OK              bool
	DurationMS      int64
	Markers         []Marker
	Screenshot      string
	ConsoleErrors   []ConsoleMessage
	NetworkFailures []NetworkFailure
	Error           string
}

type Marker struct {
	Selector string
	Count    int
	Found    bool
}

type ConsoleMessage struct {
	Route   string `json:"route,omitempty"`
	Level   string `json:"level"`
	Message string `json:"message"`
}

type NetworkFailure struct {
	Route string `json:"route,omitempty"`
	URL   string `json:"url,omitempty"`
	Type  string `json:"type,omitempty"`
	Error string `json:"error"`
}

type Artifact struct {
	Name   string
	Path   string
	Exists bool
}

func RunChecks(ctx context.Context, routes []RouteSpec, artifactRoot string, headed bool) (Result, error) {
	if err := os.MkdirAll(filepath.Join(artifactRoot, "screenshots"), 0o755); err != nil {
		return Result{}, err
	}
	allocOpts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", !headed),
		chromedp.Flag("ignore-certificate-errors", true),
		chromedp.NoFirstRun,
		chromedp.NoDefaultBrowserCheck,
	)
	allocCtx, cancelAlloc := chromedp.NewExecAllocator(ctx, allocOpts...)
	defer cancelAlloc()
	browserCtx, cancelBrowser := chromedp.NewContext(allocCtx)
	defer cancelBrowser()
	browserCtx, cancelBrowserTimeout := context.WithTimeout(browserCtx, time.Duration(len(routes))*25*time.Second)
	defer cancelBrowserTimeout()

	var mu sync.Mutex
	requestURLs := map[network.RequestID]string{}
	consoleErrors := []ConsoleMessage{}
	networkFailures := []NetworkFailure{}
	currentRoute := ""
	chromedp.ListenTarget(browserCtx, func(ev any) {
		mu.Lock()
		defer mu.Unlock()
		switch event := ev.(type) {
		case *network.EventRequestWillBeSent:
			if event.Request != nil {
				requestURLs[event.RequestID] = event.Request.URL
			}
		case *network.EventLoadingFailed:
			if event.Canceled {
				return
			}
			networkFailures = append(networkFailures, NetworkFailure{
				Route: currentRoute,
				URL:   requestURLs[event.RequestID],
				Type:  string(event.Type),
				Error: event.ErrorText,
			})
		case *cdpRuntime.EventConsoleAPICalled:
			if event.Type != cdpRuntime.APITypeError {
				return
			}
			consoleErrors = append(consoleErrors, ConsoleMessage{
				Route:   currentRoute,
				Level:   string(event.Type),
				Message: remoteObjectMessages(event.Args),
			})
		case *cdpRuntime.EventExceptionThrown:
			message := "unhandled exception"
			if event.ExceptionDetails != nil {
				message = event.ExceptionDetails.Text
				if event.ExceptionDetails.Exception != nil && event.ExceptionDetails.Exception.Description != "" {
					message = event.ExceptionDetails.Exception.Description
				}
			}
			consoleErrors = append(consoleErrors, ConsoleMessage{
				Route:   currentRoute,
				Level:   "exception",
				Message: message,
			})
		}
	})

	result := Result{}
	for _, spec := range routes {
		routeStarted := time.Now()
		mu.Lock()
		currentRoute = spec.Name
		consoleStart := len(consoleErrors)
		networkStart := len(networkFailures)
		mu.Unlock()

		route := Route{Name: spec.Name, URL: spec.Path, OK: true}
		var screenshot []byte
		err := chromedp.Run(browserCtx,
			network.Enable(),
			chromedp.Navigate(spec.Path),
			chromedp.WaitReady("body", chromedp.ByQuery),
			chromedp.Sleep(250*time.Millisecond),
		)
		if err == nil {
			for _, selector := range spec.Markers {
				var count int
				js := fmt.Sprintf("document.querySelectorAll(%q).length", selector)
				markerErr := chromedp.Run(browserCtx, chromedp.Evaluate(js, &count))
				if markerErr != nil {
					err = markerErr
					break
				}
				route.Markers = append(route.Markers, Marker{Selector: selector, Count: count, Found: count > 0})
				if count == 0 {
					route.OK = false
					route.Error = fmt.Sprintf("missing required DOM marker %s", selector)
				}
			}
		}
		screenshotPath := filepath.Join("screenshots", spec.Name+".png")
		if shotErr := chromedp.Run(browserCtx, chromedp.CaptureScreenshot(&screenshot)); shotErr == nil && len(screenshot) > 0 {
			abs := filepath.Join(artifactRoot, screenshotPath)
			if writeErr := os.WriteFile(abs, screenshot, 0o644); writeErr == nil {
				route.Screenshot = filepath.ToSlash(filepath.Join(".onlava", "harness", "ui", screenshotPath))
				result.Artifacts = append(result.Artifacts, Artifact{Name: "screenshot:" + spec.Name, Path: route.Screenshot, Exists: true})
			}
		}
		if err != nil {
			route.OK = false
			route.Error = err.Error()
		}
		mu.Lock()
		route.ConsoleErrors = append([]ConsoleMessage(nil), consoleErrors[consoleStart:]...)
		route.NetworkFailures = append([]NetworkFailure(nil), networkFailures[networkStart:]...)
		mu.Unlock()
		if len(route.ConsoleErrors) > 0 || len(route.NetworkFailures) > 0 {
			route.OK = false
		}
		route.DurationMS = time.Since(routeStarted).Milliseconds()
		result.Routes = append(result.Routes, route)
	}
	result.ConsoleErrors = consoleErrors
	result.NetworkFailures = networkFailures
	if err := writeJSONL(filepath.Join(artifactRoot, "console.jsonl"), consoleErrors); err == nil {
		result.Artifacts = append(result.Artifacts, Artifact{Name: "console", Path: ".onlava/harness/ui/console.jsonl", Exists: true})
	}
	if err := writeJSONL(filepath.Join(artifactRoot, "network.jsonl"), networkFailures); err == nil {
		result.Artifacts = append(result.Artifacts, Artifact{Name: "network", Path: ".onlava/harness/ui/network.jsonl", Exists: true})
	}
	return result, nil
}

func writeJSONL[T any](path string, items []T) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	for _, item := range items {
		if err := enc.Encode(item); err != nil {
			return err
		}
	}
	return os.WriteFile(path, buf.Bytes(), 0o644)
}

func remoteObjectMessages(args []*cdpRuntime.RemoteObject) string {
	parts := []string{}
	for _, arg := range args {
		if len(arg.Value) > 0 {
			parts = append(parts, string(arg.Value))
			continue
		}
		if arg.Description != "" {
			parts = append(parts, arg.Description)
		}
	}
	return strings.Join(parts, " ")
}
