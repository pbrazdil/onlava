package main

import "fmt"

func systemCommand(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: onlava system agent|edge|toolchain|trust ...")
	}
	switch args[0] {
	case "agent":
		return agentCommand(args[1:])
	case "edge":
		return edgeCommand(args[1:])
	case "toolchain":
		return toolchainCommand(args[1:])
	case "trust":
		opts, err := parseEdgeArgs(args[1:])
		if err != nil {
			return err
		}
		return edgeTrust(opts)
	default:
		return fmt.Errorf("unknown system command %q", args[0])
	}
}
