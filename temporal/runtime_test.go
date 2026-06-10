package temporal

import (
	"strings"
	"testing"
	"time"

	temporalinterceptor "go.temporal.io/sdk/interceptor"
	"go.temporal.io/sdk/workflow"

	sceneryruntime "scenery.sh/runtime"
)

func TestTemporalClientOptionsValidatePayloadCodec(t *testing.T) {
	_, err := temporalClientOptions(sceneryruntime.TemporalRuntimeInfo{
		Address:      sceneryruntime.DefaultTemporalAddress,
		Namespace:    sceneryruntime.DefaultTemporalNamespace,
		PayloadCodec: "custom",
	})
	if err == nil || !strings.Contains(err.Error(), "payload_codec") {
		t.Fatalf("temporalClientOptions error = %v", err)
	}
}

func TestTemporalClientOptionsAddsDevTelemetryInterceptor(t *testing.T) {
	restore := setTemporalTracingEnabledForTest(true)
	defer restore()

	options, err := temporalClientOptions(sceneryruntime.TemporalRuntimeInfo{
		Address:      sceneryruntime.DefaultTemporalAddress,
		Namespace:    sceneryruntime.DefaultTemporalNamespace,
		PayloadCodec: sceneryruntime.DefaultTemporalPayloadCodec,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(options.Interceptors) != 1 {
		t.Fatalf("interceptors = %d, want 1", len(options.Interceptors))
	}
}

func TestSceneryTemporalTracerPropagatesParent(t *testing.T) {
	tracer := newSceneryTemporalTracer(sceneryruntime.TemporalRuntimeInfo{})
	parent, err := tracer.UnmarshalSpan(map[string]string{
		"trace_id": "11111111111111111111111111111111",
		"span_id":  "2222222222222222",
	})
	if err != nil {
		t.Fatal(err)
	}
	span, err := tracer.StartSpan(&temporalinterceptor.TracerStartSpanOptions{
		Parent:    parent,
		Operation: "RunActivity",
		Name:      "agents.PlanCIFailureFix/v1",
		Time:      time.Unix(10, 0),
	})
	if err != nil {
		t.Fatal(err)
	}
	data, err := tracer.MarshalSpan(span)
	if err != nil {
		t.Fatal(err)
	}
	if data["trace_id"] != "11111111111111111111111111111111" || !isTemporalSpanID(data["span_id"]) {
		t.Fatalf("marshaled span = %#v", data)
	}
	got := span.(*sceneryTemporalSpan)
	if got.parentSpanID != "2222222222222222" || temporalTraceType(got.operation) != "TEMPORAL_ACTIVITY" {
		t.Fatalf("span = %#v", got)
	}
}

func TestTemporalTLSConfigRequiresCertAndKeyPair(t *testing.T) {
	t.Setenv("TEMPORAL_TEST_CERT", "/tmp/missing-cert.pem")
	t.Setenv("TEMPORAL_TEST_KEY", "")
	_, enabled, err := temporalTLSConfig(sceneryruntime.TemporalRuntimeInfo{
		TLSEnabled:     true,
		TLSCertFileEnv: "TEMPORAL_TEST_CERT",
		TLSKeyFileEnv:  "TEMPORAL_TEST_KEY",
	})
	if err == nil || !strings.Contains(err.Error(), "must both be set") {
		t.Fatalf("temporalTLSConfig enabled=%v error=%v", enabled, err)
	}
}

func TestTemporalWorkerOptionsEnableDeploymentVersioning(t *testing.T) {
	restore := setTemporalTracingEnabledForTest(false)
	defer restore()

	info := sceneryruntime.TemporalRuntimeInfo{
		DeploymentName: "orders-api",
		WorkerBuildID:  "sha.123",
		Versioning:     sceneryruntime.TemporalVersioningAutoUpgrade,
	}
	opts := TemporalWorkerOptions(info, "worker", "orders.go")
	if !opts.DeploymentOptions.UseVersioning {
		t.Fatal("expected worker deployment versioning")
	}
	if opts.DeploymentOptions.Version.DeploymentName != "orders-api" {
		t.Fatalf("deployment name = %q", opts.DeploymentOptions.Version.DeploymentName)
	}
	if opts.DeploymentOptions.Version.BuildID != "sha.123" {
		t.Fatalf("build id = %q", opts.DeploymentOptions.Version.BuildID)
	}
	if opts.DeploymentOptions.DefaultVersioningBehavior != workflow.VersioningBehaviorAutoUpgrade {
		t.Fatalf("versioning behavior = %v", opts.DeploymentOptions.DefaultVersioningBehavior)
	}
}

func TestTemporalWorkerOptionsAddsDevTelemetryInterceptor(t *testing.T) {
	restore := setTemporalTracingEnabledForTest(true)
	defer restore()

	opts := TemporalWorkerOptions(sceneryruntime.TemporalRuntimeInfo{}, "worker", "orders.go")
	if len(opts.Interceptors) != 1 {
		t.Fatalf("interceptors = %d, want 1", len(opts.Interceptors))
	}
}

func TestTemporalWorkerOptionsEnableHostResourceReporting(t *testing.T) {
	restore := setTemporalTracingEnabledForTest(false)
	defer restore()

	opts := TemporalWorkerOptions(sceneryruntime.TemporalRuntimeInfo{
		DeploymentName: "orders-api",
	}, "worker", "orders.go")
	if opts.SysInfoProvider == nil {
		t.Fatal("expected SysInfoProvider when host resource reporting uses default")
	}

	opts = TemporalWorkerOptions(sceneryruntime.TemporalRuntimeInfo{
		DeploymentName:   "orders-api",
		HostReporting:    false,
		HostReportingSet: true,
	}, "worker", "orders.go")
	if opts.SysInfoProvider != nil {
		t.Fatal("did not expect SysInfoProvider when host resource reporting is disabled")
	}
}

func setTemporalTracingEnabledForTest(enabled bool) func() {
	prev := temporalTracingEnabled
	temporalTracingEnabled = func() bool { return enabled }
	return func() {
		temporalTracingEnabled = prev
	}
}
