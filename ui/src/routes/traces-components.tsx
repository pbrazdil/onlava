import { Link } from "@tanstack/react-router";
import { useEffect, useMemo, useState } from "react";
import { JSONView } from "../components/json-view";
import {
  buildTraceModel,
  normalizeSpanID,
  normalizeTraceID,
  type TraceSpanEventItem,
  type TraceSpanModel,
} from "../lib/traces";
import { cn, decodeBase64Utf8, formatDurationNanos, formatTime, formatTimestamp, renderMetadataPath, tryParseJSON } from "../lib/utils";
import type { ApiCallResponse, TraceSummary } from "../lib/types";

type ReplayState = {
  method: string;
  path: string;
  payloadText: string;
};

export function BackButton({ appId }: { appId: string }) {
  return (
    <Link
      to="/$appId/requests"
      params={{ appId }}
      className="inline-flex h-8 w-8 items-center justify-center rounded-md border border-border text-sm transition-colors hover:bg-sidebar-accent hover:text-sidebar-accent-foreground"
      title="Back"
    >
      ←
    </Link>
  );
}

export function TraceTimeline({
  appId,
  traceId,
  spans,
  selectedSpanID,
  traceDurationNanos,
}: {
  appId: string;
  traceId: string;
  spans: TraceSpanModel[];
  selectedSpanID: string;
  traceDurationNanos: number;
}) {
  const domain = buildTraceTimeDomain(spans, traceDurationNanos);
  const marks = buildTimelineMarks(domain.durationNanos);

  return (
    <div className="relative flex w-full flex-col">
      <div className="grid grid-cols-[minmax(7rem,10rem)_minmax(0,1fr)] gap-3">
        <div />
        <div className="relative h-7">
          <div className="absolute bottom-0 left-0 right-0 h-px bg-border" />
          <TimelineMark percent={0} label="0ms" />
          {marks.map((mark) => (
            <TimelineMark key={mark.percent} percent={mark.percent} label={mark.label} />
          ))}
          <TimelineMark percent={100} label={formatDurationNanos(domain.durationNanos)} />
        </div>
      </div>

      <div className="mt-2 space-y-1">
        {spans.map((span) => {
          const left = percentageOffset(domain, span);
          const width = percentageWidth(domain, span);
          return (
            <Link
              key={span.id}
              to="/$appId/envs/local/traces/$traceId/$spanId"
              params={{ appId, traceId, spanId: span.rawID || span.id }}
              className={cn(
                "grid grid-cols-[minmax(7rem,10rem)_minmax(0,1fr)] items-center gap-3 rounded-sm px-1 py-1 transition-colors hover:bg-foreground/5",
                selectedSpanID === span.id && "bg-foreground/10",
              )}
            >
              <div className="min-w-0">
                <div className="truncate text-xs font-medium text-foreground">{span.title}</div>
                <div className="text-[11px] text-muted-foreground">{formatDurationNanos(traceDurationFromSpan(span))}</div>
              </div>
              <div className="relative h-5 overflow-hidden rounded-sm bg-border/25">
                <div
                  className={cn(
                    "absolute top-1/2 h-3 -translate-y-1/2 rounded-sm",
                    selectedSpanID === span.id && "ring-2 ring-background",
                  )}
                  style={{
                    left: `${left}%`,
                    width: `${width}%`,
                    minWidth: "3px",
                    backgroundColor: traceSpanColor(span),
                  }}
                  title={`${span.title} ${formatDurationNanos(traceDurationFromSpan(span))}`}
                />
              </div>
            </Link>
          );
        })}
      </div>
    </div>
  );
}

export function TimelineMark({ percent, label }: { percent: number; label: string }) {
  const isStart = percent === 0;
  const isEnd = percent === 100;
  return (
    <div
      className={cn(
        "absolute bottom-0 flex w-[90px] flex-col items-center text-[10px] text-muted-foreground sm:text-xs",
        isStart ? "left-0 items-start" : isEnd ? "right-0 items-end" : "-translate-x-1/2",
      )}
      style={isStart || isEnd ? undefined : { left: `${percent}%` }}
    >
      <span className="mb-0.5 whitespace-nowrap">{label}</span>
      <figure className="h-2 w-px bg-border" />
    </div>
  );
}

export function traceSpanColor(span: TraceSpanModel): string {
  if (span.isError) {
    return "#ef4444";
  }
  switch (span.kind) {
    case "db":
      return "#eab308";
    case "auth":
      return "#22c55e";
    default:
      return "#38bdf8";
  }
}

export function TraceSpanTree({
  appId,
  traceId,
  spans,
  selectedSpanID,
  childMap,
}: {
  appId: string;
  traceId: string;
  spans: TraceSpanModel[];
  selectedSpanID: string;
  childMap: Map<string, TraceSpanModel[]>;
}) {
  const [collapsed, setCollapsed] = useState<Record<string, boolean>>({});

  useEffect(() => {
    setCollapsed({});
  }, [traceId]);

  const roots = useMemo(
    () => spans.filter((span) => !span.parentID || !spans.some((item) => item.id === span.parentID)),
    [spans],
  );

  return (
    <div>
      {roots.map((span) => (
        <TraceSpanTreeItem
          key={span.id}
          appId={appId}
          traceId={traceId}
          span={span}
          depth={0}
          selectedSpanID={selectedSpanID}
          childMap={childMap}
          collapsed={collapsed}
          onToggle={(id) => setCollapsed((current) => ({ ...current, [id]: !current[id] }))}
        />
      ))}
    </div>
  );
}

export function TraceSpanTreeItem({
  appId,
  traceId,
  span,
  depth,
  selectedSpanID,
  childMap,
  collapsed,
  onToggle,
}: {
  appId: string;
  traceId: string;
  span: TraceSpanModel;
  depth: number;
  selectedSpanID: string;
  childMap: Map<string, TraceSpanModel[]>;
  collapsed: Record<string, boolean>;
  onToggle: (id: string) => void;
}) {
  const children = childMap.get(span.id) ?? [];
  const isCollapsed = Boolean(collapsed[span.id]);

  return (
    <div>
      <div
        className={cn(
          "flex items-stretch p-2 sm:p-4 pr-0 select-none",
          selectedSpanID === span.id ? "bg-foreground/10 font-medium" : "",
        )}
      >
        <div className="flex-shrink-0" style={{ width: `${depth * 14 + 20}px` }} />
        {children.length > 0 ? (
          <button
            type="button"
            className="bg-background z-40 h-3 w-3 -ml-[15px] mr-[3px] mt-[2.5px] flex-shrink-0 text-xs"
            onClick={() => onToggle(span.id)}
          >
            {isCollapsed ? "+" : "−"}
          </button>
        ) : (
          <div className="w-3 mr-[3px]" />
        )}
        <Link
          to="/$appId/envs/local/traces/$traceId/$spanId"
          params={{ appId, traceId, spanId: span.rawID || span.id }}
          className="flex grow flex-col ml-1 min-w-0"
        >
          <div className={cn("text-xs truncate mb-1", span.isError ? "text-destructive" : "text-foreground")}>
            {span.title}
          </div>
          <div className="mt-1 text-[11px] text-muted-foreground">
            {formatDurationNanos(span.durationNanos)} • {formatTime(span.startedAt)}
          </div>
        </Link>
      </div>
      {!isCollapsed
        ? children.map((child) => (
            <TraceSpanTreeItem
              key={child.id}
              appId={appId}
              traceId={traceId}
              span={child}
              depth={depth + 1}
              selectedSpanID={selectedSpanID}
              childMap={childMap}
              collapsed={collapsed}
              onToggle={onToggle}
            />
          ))
        : null}
    </div>
  );
}

export function SpanDetail({
  appId,
  span,
  counts,
  onOpenExplorer,
  onReplay,
  replayOpen,
}: {
  appId: string;
  span: TraceSpanModel;
  counts: ReturnType<typeof countSpanActivity> | null;
  onOpenExplorer: () => void;
  onReplay: () => void;
  replayOpen: boolean;
}) {
  const logs = span.events.filter((event) => event.kind === "log_message");
  const request = requestStartPayload(span);
  const parentTraceID = parentTraceLinkID(span);
  const requestMethod = stringField(request?.http_method);
  const requestPath = stringField(request?.path);

  return (
    <div className="flex flex-col flex-1 min-h-0 min-w-0 overflow-hidden">
      <div className="p-4 pb-0 min-w-0">
        <div className="flex flex-col sm:flex-row justify-between items-start gap-2">
          <div className="flex items-center flex-wrap">
            <h2 className="text-lg sm:text-xl font-medium break-words">{span.title}</h2>
            {request ? (
              <button
                type="button"
                onClick={onReplay}
                className="ml-3 rounded-md border border-border px-3 py-1.5 text-sm transition-colors hover:bg-sidebar-accent hover:text-sidebar-accent-foreground"
              >
                {replayOpen ? "Hide Replay" : "Replay"}
              </button>
            ) : null}
            <button
              type="button"
              onClick={onOpenExplorer}
              className="ml-2 rounded-md border border-border px-3 py-1.5 text-sm transition-colors hover:bg-sidebar-accent hover:text-sidebar-accent-foreground"
            >
              Open in API Explorer
            </button>
            <Link
              to="/$appId/envs/local/api/$serviceSlug/$rpcSlug"
              params={{ appId, serviceSlug: span.serviceName || "_", rpcSlug: span.endpointName || "_" }}
              className="ml-2 text-xs underline text-muted-foreground"
            >
              Service Catalog
            </Link>
          </div>
        </div>
        <SummaryRow span={span} counts={counts} />
      </div>

      <div className="px-4 min-w-0 min-h-0 flex-1 overflow-auto">
        <div className="mt-6 min-w-0">
          <SpanEventTimeline span={span} />
        </div>
        <div className="mt-6 grid grid-cols-2 gap-4">
          <InfoCard label="Service" value={span.serviceName || "n/a"} />
          <InfoCard label="Endpoint" value={span.endpointName || "n/a"} />
          <InfoCard label="Kind" value={span.kind} />
          <InfoCard label="Duration" value={formatDurationNanos(span.durationNanos)} />
          <InfoCard label="Started" value={formatTimestamp(span.startedAt)} />
          <InfoCard label="Ended" value={formatTimestamp(span.endedAt)} />
          <InfoCard label="Status code" value={span.statusCode || "n/a"} />
          <InfoCard label="HTTP status" value={span.httpStatusCode ? String(span.httpStatusCode) : "n/a"} />
          <InfoCard label="Method" value={requestMethod || "n/a"} />
          <InfoCard label="Path" value={requestPath || "n/a"} />
          <InfoCard label="User ID" value={span.userID || "n/a"} />
          <InfoCard label="Span ID" value={span.rawID || span.id} mono />
        </div>
        {parentTraceID ? (
          <div className="mt-4">
            <Link
              to="/$appId/envs/local/traces/$traceId"
              params={{ appId, traceId: parentTraceID }}
              className="text-sm underline"
            >
              Open parent trace
            </Link>
          </div>
        ) : null}
        <div className="mt-6 space-y-4">
          {span.start ? <JSONView title={`${span.start.kind} start`} value={span.start.payload} /> : null}
          {span.end ? <JSONView title={`${span.end.kind} end`} value={span.end.payload} /> : null}
        </div>
        {logs.length > 0 ? (
          <section className="mt-6">
            <h3 className="text-sm font-medium">Logs</h3>
            <div className="mt-4 space-y-3">
              {logs.map((event) => (
                <EventCard key={`${event.id}-${event.kind}`} event={event} />
              ))}
            </div>
          </section>
        ) : null}
        {span.events.filter((event) => event.kind !== "log_message").length > 0 ? (
          <section className="mt-6">
            <h3 className="text-sm font-medium">Events</h3>
            <div className="mt-4 space-y-3">
              {span.events
                .filter((event) => event.kind !== "log_message")
                .map((event) => (
                  <EventCard key={`${event.id}-${event.kind}`} event={event} />
                ))}
            </div>
          </section>
        ) : null}
        <div className="shrink-0 h-20" />
      </div>
    </div>
  );
}

export function SummaryRow({
  span,
  counts,
}: {
  span: TraceSpanModel;
  counts: ReturnType<typeof countSpanActivity> | null;
}) {
  return (
    <div className="text-xs flex flex-wrap mt-0.5 gap-y-1">
      <Metric label="Duration" value={formatDurationNanos(span.durationNanos)} />
      <Metric label="API Calls" value={String(counts?.requests ?? 0)} />
      <Metric label="DB Queries" value={String(counts?.db ?? 0)} />
      <Metric label="HTTP Calls" value={String(counts?.httpCalls ?? 0)} />
      <Metric label="Log Lines" value={String(counts?.logs ?? 0)} />
    </div>
  );
}

export function Metric({ label, value }: { label: string; value: string }) {
  return (
    <div className="items-center inline-flex pr-4 sm:pr-6 pt-2">
      <span className="font-semibold mr-1">{value}</span>
      {label}
    </div>
  );
}

export function SpanEventTimeline({ span }: { span: TraceSpanModel }) {
  const total = traceDurationFromSpan(span);
  const base = parseTime(span.startedAt) ?? 0;
  const items = span.events.filter((event) => event.kind !== "log_message");
  if (!items.length || total <= 0) {
    return null;
  }
  return (
    <div className="rounded-md border border-border p-4">
      <div className="text-sm font-medium">Event timeline</div>
      <div className="mt-4 relative h-8">
        <div className="absolute inset-x-0 top-1/2 h-px -translate-y-1/2 bg-border" />
        {items.map((event) => {
          const offset = ((parseTime(event.at) ?? base) - base) / total;
          return (
            <div
              key={event.id}
              className="absolute top-1/2 h-3 w-3 -translate-x-1/2 -translate-y-1/2 rounded-full"
              style={{ left: `${Math.max(0, Math.min(100, offset * 100))}%`, backgroundColor: "#38bdf8" }}
              title={`${event.title} · ${formatTime(event.at)}`}
            />
          );
        })}
      </div>
    </div>
  );
}

export function ReplayPanel({
  appId,
  state,
  onChange,
  onClose,
  loading,
  response,
  error,
  onSubmit,
}: {
  appId: string;
  state: ReplayState;
  onChange: (next: ReplayState) => void;
  onClose: () => void;
  loading: boolean;
  response: ApiCallResponse | null;
  error: string | null;
  onSubmit: () => Promise<void>;
}) {
  return (
    <div>
      <div className="flex items-center mb-4 justify-between">
        <h2 className="text-xl font-medium">Replay request</h2>
        <button
          type="button"
          className="rounded-md border border-border px-2 py-1 text-sm"
          onClick={onClose}
        >
          Close
        </button>
      </div>
      <div className="space-y-4">
        <Field
          label="Method"
          value={state.method}
          onChange={(value) => onChange({ ...state, method: value.toUpperCase() })}
        />
        <Field
          label="Path"
          value={state.path}
          onChange={(value) => onChange({ ...state, path: value })}
        />
        <TextAreaField
          label="Payload JSON"
          value={state.payloadText}
          onChange={(value) => onChange({ ...state, payloadText: value })}
        />
        <button
          type="button"
          className="rounded-md border border-border px-3 py-2 text-sm transition-colors hover:bg-sidebar-accent hover:text-sidebar-accent-foreground"
          onClick={() => void onSubmit()}
          disabled={loading}
        >
          {loading ? "Calling..." : "Send request"}
        </button>
        {error ? (
          <div className="rounded-md border border-red-500/30 bg-red-500/10 px-4 py-3 text-sm text-red-500">
            {error}
          </div>
        ) : null}
        {response ? (
          <div className="space-y-4">
            <div className="grid grid-cols-3 gap-4">
              <InfoCard label="Status" value={response.status} />
              <InfoCard label="Code" value={String(response.status_code)} />
              <InfoCard label="Trace" value={response.trace_id || "n/a"} mono />
            </div>
            {response.trace_id ? (
              <Link
                to="/$appId/envs/local/traces/$traceId"
                params={{ appId, traceId: response.trace_id }}
                className="inline-flex text-sm underline"
              >
                Open trace
              </Link>
            ) : null}
            <JSONView title="Response body" value={tryParseJSON(response.body)} />
          </div>
        ) : null}
      </div>
    </div>
  );
}

export function EventCard({ event }: { event: TraceSpanEventItem }) {
  return (
    <div className="rounded-md border border-border px-4 py-3">
      <div className="flex items-center justify-between gap-4">
        <strong className="text-sm">{event.title}</strong>
        <span className="text-xs text-muted-foreground">{formatTime(event.at)}</span>
      </div>
      <pre className="mt-3 overflow-auto whitespace-pre-wrap text-xs leading-6">
        {JSON.stringify(event.payload, null, 2)}
      </pre>
    </div>
  );
}

export function InfoCard({ label, value, mono }: { label: string; value: string; mono?: boolean }) {
  return (
    <div className="rounded-md border border-border p-4">
      <div className="text-xs uppercase tracking-wide text-muted-foreground">{label}</div>
      <div className={cn("mt-2 text-sm", mono && "font-mono break-all")}>{value || "n/a"}</div>
    </div>
  );
}

export function Field({
  label,
  value,
  onChange,
}: {
  label: string;
  value: string;
  onChange: (value: string) => void;
}) {
  return (
    <div className="space-y-2">
      <label className="text-sm font-medium">{label}</label>
      <input
        className="h-10 w-full rounded-md border border-border px-3 text-sm"
        value={value}
        onChange={(event) => onChange(event.target.value)}
      />
    </div>
  );
}

export function TextAreaField({
  label,
  value,
  onChange,
}: {
  label: string;
  value: string;
  onChange: (value: string) => void;
}) {
  return (
    <div className="space-y-2">
      <label className="text-sm font-medium">{label}</label>
      <textarea
        className="w-full rounded-md border border-border px-3 py-2 text-sm"
        style={{ minHeight: 180 }}
        value={value}
        onChange={(event) => onChange(event.target.value)}
      />
    </div>
  );
}

export function PanelDividerLine() {
  return <div className="hidden md:block w-px bg-border h-[calc(100%-40px)] absolute left-0 top-[20px]" />;
}

export function requestStartPayload(span: TraceSpanModel): Record<string, unknown> | null {
  return span.start?.kind === "request" ? span.start.payload : null;
}

export function parentTraceLinkID(span: TraceSpanModel): string {
  const request = requestStartPayload(span);
  if (!request) {
    return "";
  }
  const raw = request.parent_trace_id;
  const normalized = normalizeTraceID(raw);
  return normalized && normalized !== span.traceID ? normalized : "";
}

export function countSpanActivity(
  span: TraceSpanModel,
  childMap: Map<string, TraceSpanModel[]>,
): {
  requests: number;
  db: number;
  httpCalls: number;
  logs: number;
} {
  let requests = 0;
  let db = 0;
  let httpCalls = 0;
  let logs = 0;

  const walk = (node: TraceSpanModel) => {
    if (node !== span) {
      if (node.kind === "request") {
        requests += 1;
      }
      if (node.kind === "db") {
        db += 1;
      }
    }
    for (const event of node.events) {
      if (event.kind === "http_call_start") {
        httpCalls += 1;
      }
      if (event.kind === "log_message") {
        logs += 1;
      }
    }
    for (const child of childMap.get(node.id) ?? []) {
      walk(child);
    }
  };

  walk(span);
  return { requests, db, httpCalls, logs };
}

export type TraceTimeDomain = {
  startMs: number;
  endMs: number;
  durationNanos: number;
};

export function buildTimelineMarks(totalNanos: number): Array<{ percent: number; label: string }> {
  if (totalNanos <= 0) {
    return [];
  }
  const totalMs = totalNanos / 1_000_000;
  const step = totalMs / 5;
  const marks: Array<{ percent: number; label: string }> = [];
  for (let index = 1; index < 5; index += 1) {
    const value = step * index;
    marks.push({
      percent: (value / totalMs) * 100,
      label: value >= 1000 ? `${(value / 1000).toFixed(value >= 10_000 ? 0 : 1)}s` : `${Math.round(value)}ms`,
    });
  }
  return marks;
}

export function totalTraceDuration(model: ReturnType<typeof buildTraceModel>, summary?: TraceSummary): number {
  return summary?.duration_nanos || traceDurationFromSpan(model.rootSpan) || 0;
}

export function traceDurationFromSpan(span: TraceSpanModel | undefined): number {
  if (!span) {
    return 0;
  }
  if (span.durationNanos) {
    return span.durationNanos;
  }
  const start = parseTime(span.startedAt);
  const end = endTime(span);
  return start !== null && end !== null ? Math.max(0, (end - start) * 1_000_000) : 0;
}

export function buildTraceTimeDomain(spans: TraceSpanModel[], traceDurationNanos: number): TraceTimeDomain {
  const starts = spans
    .map((span) => spanStartTime(span))
    .filter((value): value is number => value !== null);
  const ends = spans
    .map((span) => spanEndTime(span))
    .filter((value): value is number => value !== null);

  const fallbackDurationMs = Math.max(1, traceDurationNanos / 1_000_000);
  const fallbackStart = starts[0] ?? 0;
  let startMs = starts.length > 0 ? Math.min(...starts) : fallbackStart;
  let endMs = ends.length > 0 ? Math.max(...ends) : startMs + fallbackDurationMs;

  const rootStart = spanStartTime(spans[0]);
  if (rootStart !== null && traceDurationNanos > 0) {
    startMs = Math.min(startMs, rootStart);
    endMs = Math.max(endMs, rootStart + traceDurationNanos / 1_000_000);
  }

  if (!Number.isFinite(startMs) || !Number.isFinite(endMs) || endMs <= startMs) {
    startMs = 0;
    endMs = fallbackDurationMs;
  }

  return {
    startMs,
    endMs,
    durationNanos: Math.max(1, (endMs - startMs) * 1_000_000),
  };
}

export function percentageOffset(domain: TraceTimeDomain, span: TraceSpanModel): number {
  const current = spanStartTime(span);
  if (current === null) {
    return 0;
  }
  return clampPercent(((current - domain.startMs) / (domain.endMs - domain.startMs)) * 100);
}

export function percentageWidth(domain: TraceTimeDomain, span: TraceSpanModel): number {
  const start = spanStartTime(span) ?? domain.startMs;
  const end = spanEndTime(span) ?? start;
  const clampedStart = Math.max(domain.startMs, Math.min(domain.endMs, start));
  const clampedEnd = Math.max(domain.startMs, Math.min(domain.endMs, end));
  const width = ((Math.max(clampedEnd, clampedStart) - clampedStart) / (domain.endMs - domain.startMs)) * 100;
  return Math.max(0.35, Math.min(100, width));
}

export function spanStartTime(span: TraceSpanModel | undefined): number | null {
  const explicit = parseTime(span?.startedAt);
  if (explicit !== null) {
    return explicit;
  }
  const end = endTime(span);
  if (end === null || !span?.durationNanos) {
    return null;
  }
  return end - span.durationNanos / 1_000_000;
}

export function spanEndTime(span: TraceSpanModel | undefined): number | null {
  const explicit = endTime(span);
  if (explicit !== null) {
    return explicit;
  }
  const start = parseTime(span?.startedAt);
  if (start === null) {
    return null;
  }
  return start;
}

export function clampPercent(value: number): number {
  if (!Number.isFinite(value)) {
    return 0;
  }
  return Math.max(0, Math.min(100, value));
}

export function endTime(span: TraceSpanModel | undefined): number | null {
  const explicit = parseTime(span?.endedAt);
  if (explicit !== null) {
    return explicit;
  }
  const start = parseTime(span?.startedAt);
  if (start === null || !span?.durationNanos) {
    return null;
  }
  return start + span.durationNanos / 1_000_000;
}

export function parseTime(value?: string): number | null {
  if (!value) {
    return null;
  }
  const parsed = Date.parse(value);
  return Number.isFinite(parsed) ? parsed : null;
}

export function compareDateString(a?: string, b?: string): number {
  return (parseTime(a) ?? 0) - (parseTime(b) ?? 0);
}

export function normalizeLegacyDecimalSpanID(value: string | undefined): string {
  if (!value || !/^[0-9]+$/.test(value)) {
    return "";
  }
  try {
    return normalizeSpanID(BigInt(value).toString(16));
  } catch {
    return "";
  }
}

export function pathParamsObject(pathTemplate: string, rawPathParams: unknown): Record<string, string> {
  const values = Array.isArray(rawPathParams) ? rawPathParams.map((item) => String(item ?? "")) : [];
  const keys = pathTemplate
    .split("/")
    .filter((segment) => segment.startsWith(":"))
    .map((segment) => segment.slice(1));
  const out: Record<string, string> = {};
  for (const [index, key] of keys.entries()) {
    out[key] = values[index] || "";
  }
  return out;
}

export function requestPayloadText(value: unknown): string {
  if (typeof value !== "string" || !value) {
    return "{}";
  }
  const decoded = decodeBase64Utf8(value);
  const parsed = tryParseJSON(decoded);
  return typeof parsed === "string" ? decoded : JSON.stringify(parsed, null, 2);
}

export function stringField(value: unknown): string {
  return typeof value === "string" ? value : "";
}
