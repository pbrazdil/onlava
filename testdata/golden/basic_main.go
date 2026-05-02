package main

import (
	_ "example.com/basicapp/service"
	"fmt"
	onlavaruntime "github.com/pbrazdil/onlava/runtime"
	"os"
)

func main() {
	if err := onlavaruntime.Main(onlavaruntime.AppConfig{Name: "basicapp", Workspace: "basic", ListenAddr: onlavaruntime.ListenAddrFromEnv()}); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "onlava: %v\n", err)
		os.Exit(1)
	}
}
