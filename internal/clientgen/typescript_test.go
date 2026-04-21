package clientgen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	appcfg "pulse.dev/internal/app"
	"pulse.dev/internal/parse"
)

func TestGenerateTypeScriptIncludesStructuredRequestHandling(t *testing.T) {
	appRoot := filepath.Join(appcfg.RepoRoot(), "testdata", "apps", "basic")
	model, err := parse.App(appRoot, "basicapp")
	if err != nil {
		t.Fatalf("parse app: %v", err)
	}

	out, err := GenerateTypeScript(model, TypeScriptOptions{AppSlug: "basicapp"})
	if err != nil {
		t.Fatalf("GenerateTypeScript() error = %v", err)
	}
	got := string(out)

	for _, want := range []string{
		`export namespace service {`,
		`public async Echo(name: string, params: EchoRequest): Promise<EchoResponse> {`,
		`title: encodeQueryValue(params.Title),`,
		`"X-Echo": encodeHeaderValue(params.Header),`,
		`body: encodeQueryValue(params.body),`,
		"const resp = await this.baseClient.callTypedAPI(\"GET\", `/echo/${encodeURIComponent(String(name))}`, undefined, { query, headers })",
		`public async Raw(rest: string, method: string, body?: RequestInit["body"], options?: CallParameters): Promise<globalThis.Response> {`,
		"return await this.baseClient.callAPI(method, `/raw/${encodePathWildcard(String(rest))}`, body, options)",
		`export interface EchoResponse {`,
		`message: string`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("generated client missing %q\n%s", want, got)
		}
	}
}

func TestGenerateTypeScriptIncludesNamedAliases(t *testing.T) {
	appRoot := t.TempDir()
	writeFile := func(rel, data string) {
		path := filepath.Join(appRoot, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	writeFile("go.mod", "module example.com/clientapp\n\ngo 1.26.0\n\nrequire pulse.dev v0.0.0\n\nreplace pulse.dev => "+appcfg.RepoRoot()+"\n")
	writeFile("pulse.app", `{"name":"clientapp"}`)
	writeFile("point/point.go", `package point

type Point3 struct {
	X int `+"`json:\"x\"`"+`
	Y int `+"`json:\"y\"`"+`
	Z int `+"`json:\"z\"`"+`
}
`)
	writeFile("maps/api.go", `package maps

import (
	"context"

	"example.com/clientapp/point"
)

type TaskStatus string

type Response struct {
	Status TaskStatus `+"`json:\"status\"`"+`
	Point  point.Point3 `+"`json:\"point\"`"+`
}

//pulse:api public
func Get(ctx context.Context) (*Response, error) {
	return &Response{}, nil
}
`)

	model, err := parse.App(appRoot, "clientapp")
	if err != nil {
		t.Fatalf("parse app: %v", err)
	}

	out, err := GenerateTypeScript(model, TypeScriptOptions{AppSlug: "clientapp"})
	if err != nil {
		t.Fatalf("GenerateTypeScript() error = %v", err)
	}
	got := string(out)

	for _, want := range []string{
		`export type TaskStatus = string`,
		`status: TaskStatus`,
		`export namespace point {`,
		`export interface Point3 {`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("generated client missing %q", want)
		}
	}
}
