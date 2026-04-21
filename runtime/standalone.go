package runtime

import "context"

type StandaloneDevInfo struct {
	APIURL      string
	ConsoleURL  string
	MCPBaseURL  string
	FrontendURL string
	DBStudioURL string
}

type StandaloneDevSession interface {
	Close() error
}

type StandaloneDevStarter func(context.Context, AppConfig) (StandaloneDevSession, StandaloneDevInfo, error)

var standaloneDevStarter StandaloneDevStarter

func RegisterStandaloneDevStarter(starter StandaloneDevStarter) {
	standaloneDevStarter = starter
}
