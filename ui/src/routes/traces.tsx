import { Link, useLocation, useNavigate } from "@tanstack/react-router";
import { useEffect, useMemo, useState } from "react";
import { useDashboard } from "../lib/dashboard-context";
import {
  loadPersistedTabs,
  makeTabFromEndpoint,
  persistTabs,
  type RequestTab,
} from "../lib/api-explorer";
import {
  buildTraceModel,
  normalizeTraceID,
  normalizeSpanID,
  type TraceCompatEvent,
  type TraceSpanEventItem,
  type TraceSpanModel,
} from "../lib/traces";
import { cn, formatDurationNanos, formatTime, formatTimestamp, renderMetadataPath, tryParseJSON } from "../lib/utils";
import {
  BackButton,
  PanelDividerLine,
  ReplayPanel,
  SpanDetail,
  TraceSpanTree,
  TraceTimeline,
  compareDateString,
  countSpanActivity,
  normalizeLegacyDecimalSpanID,
  parentTraceLinkID,
  pathParamsObject,
  requestPayloadText,
  requestStartPayload,
  stringField,
  totalTraceDuration,
} from "./traces-components";
import type { ApiCallResponse, EndpointOption, TraceSummary } from "../lib/types";

type ReplayState = {
  method: string;
  path: string;
  payloadText: string;
};

export function TracesPage({ traceId, spanId }: { traceId?: string; spanId?: string }) {
  const navigate = useNavigate();
  const { appId, callAPI, meta, rpc, traces } = useDashboard();
  const [events, setEvents] = useState<TraceCompatEvent[]>([]);
  const [summaries, setSummaries] = useState<TraceSummary[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);
  const [replayOpen, setReplayOpen] = useState(false);
  const [replayState, setReplayState] = useState<ReplayState | null>(null);
  const [replayResponse, setReplayResponse] = useState<ApiCallResponse | null>(null);
  const [replayError, setReplayError] = useState<string | null>(null);
  const [replayLoading, setReplayLoading] = useState(false);

  const endpointOptions = useMemo<EndpointOption[]>(
    () =>
      (meta?.svcs ?? []).flatMap((svc) =>
        svc.rpcs.map((rpcMeta) => ({
          key: `${svc.name}.${rpcMeta.name}`,
          svcName: svc.name,
          rpcName: rpcMeta.name,
          method: rpcMeta.http_methods?.[0] || "GET",
          path: renderMetadataPath(rpcMeta.path),
        })),
      ),
    [meta],
  );

  const endpointMap = useMemo(
    () => new Map(endpointOptions.map((item) => [item.key, item])),
    [endpointOptions],
  );

  useEffect(() => {
    if (!rpc || !traceId) {
      setEvents([]);
      setSummaries([]);
      setError(null);
      return;
    }
    setLoading(true);
    setError(null);
    Promise.all([
      rpc.request<TraceCompatEvent[]>("traces/get", { app_id: appId, trace_id: traceId }),
      rpc.request<TraceSummary[]>("traces/spans/summaries/list", { app_id: appId, trace_id: traceId }),
    ])
      .then(([nextEvents, nextSummaries]) => {
        setEvents(nextEvents ?? []);
        setSummaries(nextSummaries ?? []);
      })
      .catch((err: Error) => setError(err.message))
      .finally(() => setLoading(false));
  }, [appId, rpc, traceId]);

  const model = useMemo(
    () => (traceId ? buildTraceModel(traceId, summaries, events) : null),
    [events, summaries, traceId],
  );

  const childMap = useMemo(() => {
    const map = new Map<string, TraceSpanModel[]>();
    for (const span of model?.spans ?? []) {
      if (!span.parentID) {
        continue;
      }
      const bucket = map.get(span.parentID) ?? [];
      bucket.push(span);
      map.set(span.parentID, bucket);
    }
    for (const bucket of map.values()) {
      bucket.sort((a, b) => compareDateString(a.startedAt, b.startedAt));
    }
    return map;
  }, [model]);

  const selectedSpan = useMemo(() => {
    if (!model) {
      return null;
    }
    const normalized = normalizeSpanID(spanId || "");
    const legacyNormalized = normalizeLegacyDecimalSpanID(spanId);
    return (
      model.spans.find((item) => item.id === normalized || item.id === legacyNormalized || item.rawID === spanId) ||
      model.rootSpan ||
      model.spans[0] ||
      null
    );
  }, [model, spanId]);

  const selectedTraceSummary = traces.find((item) => item.trace_id === traceId) || model?.rootSpan?.summary;
  const selectedCounts = useMemo(
    () => (selectedSpan ? countSpanActivity(selectedSpan, childMap) : null),
    [childMap, selectedSpan],
  );
  const traceDurationNanos = model ? totalTraceDuration(model, selectedTraceSummary) : 0;

  useEffect(() => {
    if (!selectedSpan) {
      setReplayState(null);
      setReplayResponse(null);
      setReplayError(null);
      setReplayOpen(false);
      return;
    }
    const request = requestStartPayload(selectedSpan);
    if (!request) {
      setReplayState(null);
      setReplayResponse(null);
      setReplayError(null);
      setReplayOpen(false);
      return;
    }
    setReplayState({
      method: stringField(request.http_method) || "GET",
      path: stringField(request.path) || "/",
      payloadText: requestPayloadText(request.request_payload),
    });
    setReplayResponse(null);
    setReplayError(null);
  }, [selectedSpan]);

  if (!traceId) {
    return null;
  }

  return (
    <section className="h-[calc(100vh-var(--header-height))]">
      {loading ? (
        <div className="h-full flex items-center justify-center py-4">
          <div className="h-8 w-8 animate-spin rounded-full border-2 border-border border-t-foreground" />
        </div>
      ) : error ? (
        <div className="p-8">
          <div className="rounded-md border border-red-500/30 bg-red-500/10 px-4 py-3 text-sm text-red-500">
            {error}
          </div>
        </div>
      ) : !model || !selectedSpan ? (
        <div className="p-8 text-sm text-muted-foreground">Trace not found.</div>
      ) : (
        <div className="h-full flex flex-col min-w-0 overflow-hidden">
          <section className="flex md:flex-row flex-col items-stretch flex-1 min-h-0 overflow-auto md:overflow-hidden min-w-0">
            <div
              className={cn(
                "flex flex-col relative min-w-0 overflow-hidden w-full md:min-h-0 min-h-[50vh]",
                replayOpen ? "md:w-4/12" : "md:w-1/2",
              )}
            >
              <div className="flex flex-col px-4 pt-4 pb-0 shrink-0">
                <div className="shrink-0 md:mr-4">
                  <h1 className="text-lg md:text-xl font-medium leading-none flex flex-wrap items-center mb-2 gap-2">
                    <BackButton appId={appId} />
                    <span>Trace Details</span>
                  </h1>
                  <div className="overflow-x-auto">
                    <table className="text-xs">
                      <tbody>
                        <tr>
                          <th className="text-left text-xs text-foreground font-semibold pr-4 py-1">Duration</th>
                          <td>{formatDurationNanos(traceDurationNanos)}</td>
                        </tr>
                        <tr>
                          <th className="text-left text-xs text-foreground font-semibold pr-4 py-1">Recorded</th>
                          <td>{formatTimestamp(selectedTraceSummary?.started_at || model.rootSpan?.startedAt)}</td>
                        </tr>
                        <tr>
                          <th className="text-left text-xs text-foreground font-semibold pr-4 py-1">Trace ID</th>
                          <td className="font-mono text-[11px]">{traceId}</td>
                        </tr>
                        {model.userID ? (
                          <tr>
                            <th className="text-left text-xs text-foreground font-semibold pr-4 py-1">User ID</th>
                            <td>{model.userID}</td>
                          </tr>
                        ) : null}
                      </tbody>
                    </table>
                  </div>
                </div>
              </div>

              <div className="flex min-h-0 w-full flex-1 flex-col overflow-auto px-4 pb-8 pt-4">
                <TraceTimeline
                  appId={appId}
                  traceId={model.traceID}
                  spans={model.spans}
                  selectedSpanID={selectedSpan.id}
                  traceDurationNanos={traceDurationNanos}
                />
              </div>
            </div>

            <div
              className={cn(
                "relative min-w-0 w-full md:min-h-0 min-h-[50vh] md:border-l-0 border-t md:border-t-0 flex flex-col overflow-hidden",
                replayOpen ? "md:w-5/12" : "md:w-1/2",
              )}
            >
              <PanelDividerLine />
              <SpanDetail
                appId={appId}
                span={selectedSpan}
                counts={selectedCounts}
                onOpenExplorer={() => void openInAPIExplorer(selectedSpan)}
                onReplay={() => setReplayOpen((value) => !value)}
                replayOpen={replayOpen}
              />
            </div>

            {replayOpen ? (
              <div className="overflow-hidden h-full md:h-full h-auto md:min-h-0 min-h-[50vh] md:w-3/12 w-full">
                <div className="p-4 relative w-full md:min-w-[350px] h-full overflow-auto">
                  <PanelDividerLine />
                  {replayState ? (
                    <ReplayPanel
                      appId={appId}
                      state={replayState}
                      onChange={setReplayState}
                      onClose={() => setReplayOpen(false)}
                      loading={replayLoading}
                      response={replayResponse}
                      error={replayError}
                      onSubmit={async () => {
                        if (!replayState) {
                          return;
                        }
                        await replaySelectedSpan(selectedSpan, replayState);
                      }}
                    />
                  ) : null}
                </div>
              </div>
            ) : null}
          </section>
        </div>
      )}
    </section>
  );

  async function replaySelectedSpan(span: TraceSpanModel, state: ReplayState) {
    const request = requestStartPayload(span);
    if (!request) {
      return;
    }
    setReplayLoading(true);
    setReplayError(null);
    setReplayResponse(null);
    try {
      const result = await callAPI({
        service: stringField(request.service_name) || span.serviceName,
        endpoint: stringField(request.endpoint_name) || span.endpointName,
        path: state.path,
        method: state.method,
        payload: tryParseJSON(state.payloadText),
      });
      setReplayResponse(result);
    } catch (err) {
      setReplayError(err instanceof Error ? err.message : String(err));
    } finally {
      setReplayLoading(false);
    }
  }

  async function openInAPIExplorer(span: TraceSpanModel) {
    const request = requestStartPayload(span);
    if (!request) {
      return;
    }

    const serviceName = stringField(request.service_name) || span.serviceName;
    const endpointName = stringField(request.endpoint_name) || span.endpointName;
    if (!serviceName || !endpointName) {
      return;
    }

    const key = `${serviceName}.${endpointName}`;
    const endpoint =
      endpointMap.get(key) ||
      ({
        key,
        svcName: serviceName,
        rpcName: endpointName,
        method: stringField(request.http_method) || "GET",
        path: stringField(request.path) || "/",
      } satisfies EndpointOption);

    const next = makeTabFromEndpoint(endpoint, key);
    next.method = stringField(request.http_method) || endpoint.method;
    next.path = stringField(request.path) || endpoint.path;
    next.pathParamsText = JSON.stringify(pathParamsObject(endpoint.path, request.path_params), null, 2);
    next.payloadText = requestPayloadText(request.request_payload);

    const persisted = loadPersistedTabs(appId);
    const tabs: RequestTab[] = [...persisted.tabs, next];
    persistTabs(appId, next.id, tabs);
    await navigate({ to: "/$appId/requests", params: { appId } });
  }
}

export function TracesListPage() {
  const navigate = useNavigate();
  const location = useLocation();
  const { appId, rpc, traces, refreshAll } = useDashboard();
  const searchParams = useMemo(() => new URLSearchParams(location.search), [location.search]);
  const [traceServiceFilter, setTraceServiceFilter] = useState(searchParams.get("service") ?? "");
  const [traceEndpointFilter, setTraceEndpointFilter] = useState(searchParams.get("endpoint") ?? "");
  const [traceStatusFilter, setTraceStatusFilter] = useState<"all" | "error">(
    searchParams.get("error") === "true" ? "error" : "all",
  );
  const [traceIDFilter, setTraceIDFilter] = useState(searchParams.get("trace_id") ?? "");

  useEffect(() => {
    setTraceServiceFilter(searchParams.get("service") ?? "");
    setTraceEndpointFilter(searchParams.get("endpoint") ?? "");
    setTraceStatusFilter(searchParams.get("error") === "true" ? "error" : "all");
    setTraceIDFilter(searchParams.get("trace_id") ?? "");
  }, [searchParams]);

  const traceServices = useMemo(
    () => Array.from(new Set(traces.map((trace) => trace.service_name).filter(Boolean))).sort(),
    [traces],
  );
  const traceEndpoints = useMemo(
    () =>
      Array.from(
        new Set(
          traces
            .filter((trace) => !traceServiceFilter || trace.service_name === traceServiceFilter)
            .map((trace) => trace.endpoint_name)
            .filter((endpoint): endpoint is string => typeof endpoint === "string" && endpoint.length > 0),
        ),
      ).sort(),
    [traceServiceFilter, traces],
  );
  const filteredTraces = useMemo(
    () =>
      traces.filter((trace) => {
        if (traceServiceFilter && trace.service_name !== traceServiceFilter) {
          return false;
        }
        if (traceEndpointFilter && trace.endpoint_name !== traceEndpointFilter) {
          return false;
        }
        if (traceStatusFilter === "error" && !trace.is_error) {
          return false;
        }
        if (traceIDFilter && !trace.trace_id.includes(traceIDFilter.trim())) {
          return false;
        }
        return true;
      }),
    [traceEndpointFilter, traceIDFilter, traceServiceFilter, traceStatusFilter, traces],
  );

  useEffect(() => {
    const next = new URLSearchParams();
    if (traceServiceFilter) {
      next.set("service", traceServiceFilter);
    }
    if (traceEndpointFilter) {
      next.set("endpoint", traceEndpointFilter);
    }
    if (traceStatusFilter === "error") {
      next.set("error", "true");
    }
    if (traceIDFilter) {
      next.set("trace_id", traceIDFilter);
    }
    const nextSearch = next.toString();
    const currentSearch =
      typeof window !== "undefined" ? window.location.search.replace(/^\?/, "") : "";
    if (nextSearch !== currentSearch) {
      void navigate({
        to: "/$appId/envs/local/traces",
        params: { appId },
        search: nextSearch ? `?${nextSearch}` : "",
        replace: true,
      });
    }
  }, [appId, location.search, navigate, traceEndpointFilter, traceIDFilter, traceServiceFilter, traceStatusFilter]);

  return (
    <div className="h-[calc(100vh-var(--header-height))] overflow-auto">
      <div className="px-8 py-6">
        <div className="flex items-center justify-between gap-4">
          <h1 className="text-lg font-medium">Traces</h1>
          <button
            type="button"
            className="rounded-md border border-border px-3 py-2 text-sm transition-colors hover:bg-sidebar-accent hover:text-sidebar-accent-foreground disabled:opacity-50"
            disabled={traces.length === 0}
            onClick={() => void rpc?.request("traces/clear", { app_id: appId }).then(() => refreshAll())}
          >
            Clear traces
          </button>
        </div>

        <div className="mt-6 rounded-md border border-border p-4 space-y-4 devdash-trace-filters">
          <div className="grid grid-cols-[240px_minmax(300px,1fr)_auto] gap-6 items-end">
            <div className="space-y-2">
              <div className="text-xs font-medium uppercase tracking-wide text-muted-foreground">Service</div>
              <select
                className="h-9 w-full rounded-md border border-border px-3 text-sm"
                value={traceServiceFilter}
                onChange={(event) => {
                  setTraceServiceFilter(event.target.value);
                  setTraceEndpointFilter("");
                }}
              >
                <option value="">All services</option>
                {traceServices.map((service) => (
                  <option key={service} value={service}>
                    {service}
                  </option>
                ))}
              </select>
            </div>

            <div className="space-y-2">
              <div className="text-xs font-medium uppercase tracking-wide text-muted-foreground">Endpoint</div>
              <select
                className="h-9 w-full rounded-md border border-border px-3 text-sm"
                value={traceEndpointFilter}
                onChange={(event) => setTraceEndpointFilter(event.target.value)}
              >
                <option value="">All endpoints</option>
                {traceEndpoints.map((endpoint) => (
                  <option key={endpoint} value={endpoint}>
                    {endpoint}
                  </option>
                ))}
              </select>
            </div>

            <div className="inline-flex gap-0.5 p-1 rounded-lg bg-sidebar-accent/50 self-start">
              <button
                type="button"
                onClick={() => setTraceStatusFilter("all")}
                className={cn(
                  "px-3 py-1.5 text-sm rounded-md transition-colors",
                  traceStatusFilter === "all"
                    ? "bg-background text-foreground shadow-sm"
                    : "text-muted-foreground hover:text-foreground",
                )}
              >
                All
              </button>
              <button
                type="button"
                onClick={() => setTraceStatusFilter("error")}
                className={cn(
                  "px-3 py-1.5 text-sm rounded-md transition-colors",
                  traceStatusFilter === "error"
                    ? "bg-background text-foreground shadow-sm"
                    : "text-muted-foreground hover:text-foreground",
                )}
              >
                Errors
              </button>
            </div>
          </div>

          <div className="space-y-2 max-w-[320px]">
            <div className="text-xs font-medium uppercase tracking-wide text-muted-foreground">Trace ID</div>
            <input
              className="h-9 w-full rounded-md border border-border px-3 text-sm"
              placeholder="Trace ID"
              value={traceIDFilter}
              onChange={(event) => setTraceIDFilter(event.target.value)}
            />
          </div>
        </div>

        <div className="mt-6">
          {filteredTraces.length === 0 ? (
            <div className="rounded-md border border-border p-6 text-sm text-muted-foreground">
              No traces match the current filters.
            </div>
          ) : (
            <div className="overflow-auto rounded-md border border-border">
              <table className="min-w-full text-sm">
                <thead className="bg-muted/50">
                  <tr>
                    <th className="px-4 py-3 text-left">Request</th>
                    <th className="px-4 py-3 text-left">Status</th>
                    <th className="px-4 py-3 text-left">Recorded</th>
                    <th className="px-4 py-3 text-left">Duration</th>
                    <th className="px-4 py-3 text-left" />
                  </tr>
                </thead>
                <tbody>
                  {filteredTraces.map((trace) => {
                    const hasEndpointFilterShortcut =
                      Boolean(trace.service_name) &&
                      Boolean(trace.endpoint_name) &&
                      !(traceServiceFilter === trace.service_name && traceEndpointFilter === trace.endpoint_name);
                    return (
                      <tr
                        key={`${trace.trace_id}/${trace.span_id}`}
                        className="group cursor-pointer border-t border-border hover:bg-sidebar-accent/50"
                        onClick={(event) => {
                          const target = event.target as HTMLElement;
                          if (target.closest("a") || target.closest("button")) {
                            return;
                          }
                          void navigate({
                            to: "/$appId/envs/local/traces/$traceId",
                            params: { appId, traceId: trace.trace_id },
                          });
                        }}
                      >
                        <td className="px-4 py-3 w-1/2">
                          <Link
                            to="/$appId/envs/local/traces/$traceId"
                            params={{ appId, traceId: trace.trace_id }}
                            className="flex items-start h-full no-underline text-foreground"
                          >
                            <div className="flex flex-col min-w-0">
                              <span className="text-sm truncate group-hover:underline">
                                {trace.service_name || "unknown service"}.{trace.endpoint_name || trace.type}
                              </span>
                              <span className="text-xs text-muted-foreground font-mono truncate">
                                {trace.trace_id}
                              </span>
                            </div>
                          </Link>
                        </td>
                        <td className="px-4 py-3">
                          <span className={cn(
                            "inline-flex rounded-md px-2 py-1 text-xs",
                            trace.is_error ? "bg-red-500/10 text-red-500" : "bg-green-500/10 text-green-600",
                          )}>
                            {trace.is_error ? "Error" : "Success"}
                          </span>
                        </td>
                        <td className="px-4 py-3">
                          <span>{formatTime(trace.started_at)}</span>
                          <span className="ml-2 text-xs text-muted-foreground hidden sm:inline">
                            {formatTimestamp(trace.started_at)}
                          </span>
                        </td>
                        <td className="px-4 py-3">{formatDurationNanos(trace.duration_nanos)}</td>
                        <td className="px-4 py-3">
                          {hasEndpointFilterShortcut ? (
                            <button
                              type="button"
                              className="text-xs underline"
                              onClick={() => {
                                setTraceServiceFilter(trace.service_name || "");
                                setTraceEndpointFilter(trace.endpoint_name || "");
                              }}
                            >
                              Filter
                            </button>
                          ) : null}
                        </td>
                      </tr>
                    );
                  })}
                </tbody>
              </table>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
