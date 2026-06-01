package temporal

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"strings"
	"sync"

	temporalclient "go.temporal.io/sdk/client"
	"go.temporal.io/sdk/contrib/sysinfo"
	temporalinterceptor "go.temporal.io/sdk/interceptor"
	temporalworker "go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"

	"github.com/pbrazdil/onlava/internal/envpolicy"
	onlavaruntime "github.com/pbrazdil/onlava/runtime"
)

type temporalRuntimeState struct {
	client temporalclient.Client
	info   onlavaruntime.TemporalRuntimeInfo
}

var activeTemporal struct {
	mu    sync.RWMutex
	state *temporalRuntimeState
}

var temporalTracingEnabled = onlavaruntime.TemporalTracingEnabled

func StartRuntime(ctx context.Context, cfg onlavaruntime.AppConfig) (func(context.Context) error, error) {
	info := onlavaruntime.ResolveTemporalConfig(cfg.Name, cfg.Temporal)
	if !info.Enabled {
		return func(context.Context) error { return nil }, nil
	}
	if err := onlavaruntime.ValidateTemporalVersioning(info); err != nil {
		return nil, err
	}
	client, err := Dial(ctx, info)
	if err != nil {
		return nil, err
	}
	state := &temporalRuntimeState{
		client: client,
		info:   info,
	}
	activeTemporal.mu.Lock()
	activeTemporal.state = state
	activeTemporal.mu.Unlock()
	return func(context.Context) error {
		activeTemporal.mu.Lock()
		if activeTemporal.state == state {
			activeTemporal.state = nil
		}
		activeTemporal.mu.Unlock()
		client.Close()
		return nil
	}, nil
}

func ActiveClient() (temporalclient.Client, onlavaruntime.TemporalRuntimeInfo, bool) {
	activeTemporal.mu.RLock()
	defer activeTemporal.mu.RUnlock()
	if activeTemporal.state == nil {
		return nil, onlavaruntime.TemporalRuntimeInfo{}, false
	}
	return activeTemporal.state.client, activeTemporal.state.info, true
}

func CheckConnection(ctx context.Context, appName string, cfg onlavaruntime.TemporalConfig) (onlavaruntime.TemporalRuntimeInfo, onlavaruntime.TemporalConnectionStatus) {
	info := onlavaruntime.ResolveTemporalConfig(appName, cfg)
	if !info.Enabled {
		return info, onlavaruntime.TemporalConnectionStatus{}
	}
	client, err := Dial(ctx, info)
	if err != nil {
		return info, onlavaruntime.TemporalConnectionStatus{
			Checked: true,
			Error:   err.Error(),
		}
	}
	client.Close()
	return info, onlavaruntime.TemporalConnectionStatus{
		Checked:   true,
		Reachable: true,
	}
}

func Dial(ctx context.Context, info onlavaruntime.TemporalRuntimeInfo) (temporalclient.Client, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	options, err := temporalClientOptions(info)
	if err != nil {
		return nil, err
	}
	dialCtx, cancel := context.WithTimeout(ctx, onlavaruntime.DefaultTemporalConnectWait)
	defer cancel()
	client, err := temporalclient.DialContext(dialCtx, options)
	if err != nil {
		return nil, fmt.Errorf("temporal: connect to %s namespace %s: %w", info.Address, info.Namespace, err)
	}
	return client, nil
}

func temporalClientOptions(info onlavaruntime.TemporalRuntimeInfo) (temporalclient.Options, error) {
	if err := onlavaruntime.ValidateTemporalPayloadCodec(info.PayloadCodec); err != nil {
		return temporalclient.Options{}, err
	}
	options := temporalclient.Options{
		HostPort:  info.Address,
		Namespace: info.Namespace,
		Identity:  temporalIdentity(info),
	}
	if apiKey, ok := envValue(info.APIKeyEnv); ok {
		options.Credentials = temporalclient.NewAPIKeyStaticCredentials(apiKey)
	}
	tlsConfig, enabled, err := temporalTLSConfig(info)
	if err != nil {
		return temporalclient.Options{}, err
	}
	if enabled {
		options.ConnectionOptions.TLS = tlsConfig
	}
	if temporalTracingEnabled() {
		options.Interceptors = append(options.Interceptors, temporalinterceptor.NewTracingInterceptor(newOnlavaTemporalTracer(info)))
	}
	return options, nil
}

func temporalTLSConfig(info onlavaruntime.TemporalRuntimeInfo) (*tls.Config, bool, error) {
	caPath, caSet := envValue(info.TLSCACertFileEnv)
	certPath, certSet := envValue(info.TLSCertFileEnv)
	keyPath, keySet := envValue(info.TLSKeyFileEnv)
	enabled := info.TLSEnabled || info.TLSServerNameSet || caSet || certSet || keySet
	if !enabled {
		return nil, false, nil
	}
	cfg := &tls.Config{
		MinVersion: tls.VersionTLS12,
		ServerName: strings.TrimSpace(info.TLSServerName),
	}
	if caSet {
		pem, err := os.ReadFile(caPath)
		if err != nil {
			return nil, false, fmt.Errorf("temporal: read TLS CA certificate from %s: %w", info.TLSCACertFileEnv, err)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(pem) {
			return nil, false, fmt.Errorf("temporal: TLS CA certificate from %s does not contain PEM certificates", info.TLSCACertFileEnv)
		}
		cfg.RootCAs = pool
	}
	if certSet != keySet {
		return nil, false, fmt.Errorf("temporal: TLS client certificate and key must both be set with %s and %s", info.TLSCertFileEnv, info.TLSKeyFileEnv)
	}
	if certSet {
		cert, err := tls.LoadX509KeyPair(certPath, keyPath)
		if err != nil {
			return nil, false, fmt.Errorf("temporal: load TLS client certificate from %s/%s: %w", info.TLSCertFileEnv, info.TLSKeyFileEnv, err)
		}
		cfg.Certificates = []tls.Certificate{cert}
	}
	return cfg, true, nil
}

func temporalIdentity(info onlavaruntime.TemporalRuntimeInfo) string {
	pid := os.Getpid()
	if info.TaskQueuePrefix == "" {
		return fmt.Sprintf("onlava:%d", pid)
	}
	return fmt.Sprintf("%s:%d", info.TaskQueuePrefix, pid)
}

func TemporalWorkerOptions(info onlavaruntime.TemporalRuntimeInfo, role, taskQueue string) temporalworker.Options {
	buildID := onlavaruntime.TemporalWorkerBuildID(info)
	opts := temporalworker.Options{
		DisableRegistrationAliasing: true,
		Identity:                    onlavaruntime.TemporalWorkerIdentity(info, role, taskQueue),
		BuildID:                     buildID,
		DeploymentOptions: temporalworker.DeploymentOptions{
			UseVersioning: true,
			Version: temporalworker.WorkerDeploymentVersion{
				DeploymentName: onlavaruntime.TemporalDeploymentName(info),
				BuildID:        buildID,
			},
			DefaultVersioningBehavior: TemporalWorkflowVersioningBehavior(info),
		},
	}
	if onlavaruntime.TemporalHostResourceReportingEnabled(info) {
		opts.SysInfoProvider = sysinfo.SysInfoProvider()
	}
	if temporalTracingEnabled() {
		opts.Interceptors = append(opts.Interceptors, temporalinterceptor.NewTracingInterceptor(newOnlavaTemporalTracer(info)))
	}
	return opts
}

func TemporalWorkflowVersioningBehavior(info onlavaruntime.TemporalRuntimeInfo) workflow.VersioningBehavior {
	switch onlavaruntime.NormalizeTemporalVersioning(info.Versioning) {
	case onlavaruntime.TemporalVersioningAutoUpgrade:
		return workflow.VersioningBehaviorAutoUpgrade
	default:
		return workflow.VersioningBehaviorPinned
	}
}

func EnsureWorkerDeploymentCurrentVersion(ctx context.Context, client temporalclient.Client, info onlavaruntime.TemporalRuntimeInfo) error {
	if client == nil {
		return fmt.Errorf("temporal: missing client for worker deployment versioning")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	updateCtx, cancel := context.WithTimeout(ctx, onlavaruntime.DefaultTemporalConnectWait)
	defer cancel()
	deploymentName := onlavaruntime.TemporalDeploymentName(info)
	buildID := onlavaruntime.TemporalWorkerBuildID(info)
	_, err := client.WorkerDeploymentClient().GetHandle(deploymentName).SetCurrentVersion(updateCtx, temporalclient.WorkerDeploymentSetCurrentVersionOptions{
		BuildID:                 buildID,
		Identity:                temporalIdentity(info),
		IgnoreMissingTaskQueues: true,
		AllowNoPollers:          true,
	})
	if err != nil {
		return fmt.Errorf("temporal: set worker deployment %s current version %s: %w", deploymentName, buildID, err)
	}
	return nil
}

func envValue(name string) (string, bool) {
	if name == "" {
		return "", false
	}
	value, ok := envpolicy.Lookup(name)
	if !ok {
		return "", false
	}
	value = strings.TrimSpace(value)
	if value == "" {
		return "", false
	}
	return value, true
}
