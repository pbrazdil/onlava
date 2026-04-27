import { Link } from "@tanstack/react-router";
import { useEffect, useMemo, useState } from "react";
import { useDashboard } from "../lib/dashboard-context";
import { cn, formatTimestamp } from "../lib/utils";
import type {
  PubSubHistoryPoint,
  PubSubMessage,
  PubSubMessageAttempt,
  PubSubMessagesResponse,
  PubSubSnapshot,
  PubSubSubscription,
  PubSubTopic,
} from "../lib/types";

const chartPeriods = [
  { label: "5m", value: "5m" },
  { label: "15m", value: "15m" },
  { label: "1h", value: "1h" },
  { label: "6h", value: "6h" },
  { label: "24h", value: "24h" },
] as const;

export function PubSubPage() {
  const { appId, pubsub, rpc } = useDashboard();
  const [period, setPeriod] = useState<(typeof chartPeriods)[number]["value"]>("15m");
  const [periodSnapshot, setPeriodSnapshot] = useState<PubSubSnapshot | null>(null);
  const [messages, setMessages] = useState<PubSubMessage[]>([]);
  const [messagesLoading, setMessagesLoading] = useState(false);
  const [messagesError, setMessagesError] = useState<string | null>(null);
  const [viewMode, setViewMode] = useState<"messages" | "dlq">("messages");
  const [expandedRow, setExpandedRow] = useState<string | null>(null);
  const [attemptsByRow, setAttemptsByRow] = useState<Record<string, PubSubMessageAttempt[]>>({});
  const [topicFilter, setTopicFilter] = useState("");
  const [queueFilter, setQueueFilter] = useState("");
  const [statusFilter, setStatusFilter] = useState("");
  const [clearing, setClearing] = useState(false);
  const [clearError, setClearError] = useState<string | null>(null);
  const activeSnapshot = periodSnapshot ?? pubsub;
  const topics = activeSnapshot?.topics ?? [];
  const totals = useMemo(() => summarize(topics), [topics]);
  const availableTopics = useMemo(
    () => Array.from(new Set([...topics.map((item) => item.name), ...messages.map((item) => item.topic_name)])).sort(),
    [messages, topics],
  );
  const availableQueues = useMemo(() => {
    const names = new Set<string>();
    for (const topic of topics) {
      if (topicFilter && topic.name !== topicFilter) {
        continue;
      }
      for (const sub of topic.subscriptions) {
        names.add(sub.name);
      }
    }
    for (const message of messages) {
      if (topicFilter && message.topic_name !== topicFilter) {
        continue;
      }
      names.add(message.subscription_name);
    }
    return Array.from(names).sort();
  }, [messages, topicFilter, topics]);

  useEffect(() => {
    let cancelled = false;
    async function refreshPeriod() {
      if (!rpc) {
        return;
      }
      const next = await rpc.request<PubSubSnapshot>("pubsub/status", { app_id: appId, period });
      if (!cancelled) {
        setPeriodSnapshot(next);
      }
    }
    void refreshPeriod().catch(() => undefined);
    return () => {
      cancelled = true;
    };
  }, [appId, period, rpc]);

  useEffect(() => {
    let cancelled = false;
    async function refreshMessages() {
      if (!rpc) {
        return;
      }
      setMessagesLoading(true);
      try {
        const next = await rpc.request<PubSubMessagesResponse>("pubsub/messages", {
          app_id: appId,
          period,
          topic_name: topicFilter,
          queue_name: queueFilter,
          status: viewMode === "dlq" ? "dead_lettered" : statusFilter,
          limit: 500,
        });
        if (!cancelled) {
          setMessages(next.messages ?? []);
          setMessagesError(null);
        }
      } catch (err) {
        if (!cancelled) {
          setMessagesError(err instanceof Error ? err.message : String(err));
        }
      } finally {
        if (!cancelled) {
          setMessagesLoading(false);
        }
      }
    }
    void refreshMessages();
    return () => {
      cancelled = true;
    };
  }, [appId, period, queueFilter, rpc, statusFilter, topicFilter, viewMode]);

  useEffect(() => {
    if (!rpc) {
      return;
    }
    const unsubscribe = rpc.subscribe((notification) => {
      if (notification.method === "pubsub/update") {
        const params = notification.params as PubSubSnapshot;
        if (params.app_id === appId) {
          setPeriodSnapshot((current) => mergeLiveSnapshot(current, params));
        }
      }
      if (notification.method === "pubsub/message") {
        const message = notification.params as PubSubMessage;
        if (message.app_id !== appId) {
          return;
        }
        if (messageWithinFilters(message, period, topicFilter, queueFilter, viewMode === "dlq" ? "dead_lettered" : statusFilter)) {
          setMessages((current) => upsertMessage(current, message));
        } else {
          setMessages((current) => current.filter((item) => messageRowKey(item) !== messageRowKey(message)));
        }
        if (message.attempt && message.attempt > 0) {
          setAttemptsByRow((current) => {
            const key = messageRowKey(message);
            const existing = current[key];
            if (!existing) {
              return current;
            }
            const attempt = messageToAttempt(message);
            return {
              ...current,
              [key]: upsertAttempt(existing, attempt),
            };
          });
        }
      }
      if (notification.method === "pubsub/messages/cleared") {
        const params = notification.params as { app_id?: string };
        if (params.app_id === appId) {
          setMessages((current) =>
            current.map((message) =>
              message.status === "queued" || message.status === "processing" || message.status === "retrying"
                ? {
                    ...message,
                    status: "cleared",
                    result: { status: "cleared" },
                    finished_at: new Date().toISOString(),
                  }
                : message,
            ),
          );
        }
      }
    });
    return unsubscribe;
  }, [appId, period, queueFilter, rpc, statusFilter, topicFilter, viewMode]);

  useEffect(() => {
    if (!pubsub) {
      return;
    }
    setPeriodSnapshot((current) => mergeLiveSnapshot(current, pubsub));
  }, [pubsub]);

  useEffect(() => {
    if (topicFilter && !availableTopics.includes(topicFilter)) {
      setTopicFilter("");
    }
  }, [availableTopics, topicFilter]);

  useEffect(() => {
    if (queueFilter && !availableQueues.includes(queueFilter)) {
      setQueueFilter("");
    }
  }, [availableQueues, queueFilter]);

  useEffect(() => {
    if (!rpc || !expandedRow) {
      return;
    }
    const rpcClient = rpc;
    const rowKey = expandedRow;
    const target = messages.find((item) => messageRowKey(item) === rowKey);
    if (!target) {
      return;
    }
    const targetMessage = target;
    let cancelled = false;
    async function loadAttempts() {
      const next = await rpcClient.request<{ attempts: PubSubMessageAttempt[] }>("pubsub/message/attempts", {
        app_id: appId,
        message_id: targetMessage.message_id,
        subscription_name: targetMessage.subscription_name,
      });
      if (!cancelled) {
        setAttemptsByRow((current) => ({
          ...current,
          [rowKey]: next.attempts ?? [],
        }));
      }
    }
    void loadAttempts().catch(() => undefined);
    return () => {
      cancelled = true;
    };
  }, [appId, expandedRow, messages, rpc]);

  async function clearQueues() {
    if (!rpc || clearing) {
      return;
    }
    const confirmed = window.confirm(
      "Clear all queued Pub/Sub jobs from the local embedded NATS runtime? In-flight handlers may continue running.",
    );
    if (!confirmed) {
      return;
    }
    setClearing(true);
    setClearError(null);
    try {
      const next = await rpc.request<PubSubSnapshot>("pubsub/clear", { app_id: appId });
      setPeriodSnapshot(next);
    } catch (err) {
      setClearError(err instanceof Error ? err.message : String(err));
    } finally {
      setClearing(false);
    }
  }

  return (
    <div className="max-h-[calc(100vh-var(--header-height))] overflow-auto">
      <div className="min-h-0 grow px-8 pt-6 pb-12 leading-6">
        <div className="max-w-7xl space-y-8">
          <div className="flex items-start justify-between gap-4">
            <div>
              <h1 className="text-lg font-medium">Pub/Sub</h1>
              <p className="mt-2 max-w-3xl text-sm text-muted-foreground">
                Live local queue and worker metrics from Pulse&apos;s embedded NATS runtime.
              </p>
            </div>
            <div className="flex flex-col items-end gap-3">
              <div className="text-right text-xs text-muted-foreground">
                <div>Last update</div>
                <div className="mt-1 text-foreground">
                  {pubsub?.updated_at ? new Date(pubsub.updated_at).toLocaleTimeString() : "none"}
                </div>
              </div>
              <button
                type="button"
                onClick={() => void clearQueues()}
                disabled={!rpc || clearing || topics.length === 0}
                className="rounded-md border border-red-950/80 bg-red-950/20 px-3 py-1.5 text-xs font-medium text-red-300 hover:border-red-700 hover:text-red-200 disabled:cursor-not-allowed disabled:opacity-40"
              >
                {clearing ? "Clearing..." : "Clear queued jobs"}
              </button>
              <div className="flex rounded-md border border-border bg-sidebar/60 p-1">
                {chartPeriods.map((item) => (
                  <button
                    key={item.value}
                    type="button"
                    onClick={() => setPeriod(item.value)}
                    className={
                      item.value === period
                        ? "rounded-sm bg-foreground px-3 py-1 text-xs font-medium text-background"
                        : "rounded-sm px-3 py-1 text-xs text-muted-foreground hover:text-foreground"
                    }
                  >
                    {item.label}
                  </button>
                ))}
              </div>
            </div>
          </div>

          <div className="flex flex-wrap items-center gap-x-8 gap-y-3 rounded-md border border-border bg-sidebar/20 px-4 py-3">
            <InlineStat label="Topics" value={String(topics.length)} />
            <InlineStat label="Subscriptions" value={String(totals.subscriptions)} />
            <InlineStat label="Queued" value={formatNumber(totals.pending)} />
            <InlineStat label="Picked up" value={formatNumber(totals.pickedUp)} />
            <InlineStat label="In flight" value={formatNumber(totals.inFlight)} />
            <InlineStat label="Avg job" value={formatMillis(totals.avgDurationMs)} />
          </div>
          {clearError ? <div className="text-sm text-red-400">{clearError}</div> : null}

          {topics.length === 0 ? (
            <div className="rounded-md border border-border p-6 text-sm text-muted-foreground">
              No Pub/Sub topics have been reported yet. Start the app with packages that define{" "}
              <code>pubsub.NewTopic</code> and publish or receive messages to populate live metrics.
            </div>
          ) : (
            <div className="space-y-6">
              {topics.map((topic) => (
                <TopicCard
                  key={topic.name}
                  topic={topic}
                  history={activeSnapshot?.history ?? []}
                  latest={activeSnapshot}
                  period={period}
                />
              ))}
            </div>
          )}

          <section className="rounded-md border border-border p-6">
            <div className="flex flex-wrap items-start justify-between gap-4">
              <div>
                <div className="flex items-center gap-2">
                  <button
                    type="button"
                    onClick={() => setViewMode("messages")}
                    className={
                      viewMode === "messages"
                        ? "rounded-md bg-foreground px-3 py-1.5 text-sm font-medium text-background"
                        : "rounded-md border border-border px-3 py-1.5 text-sm text-muted-foreground hover:text-foreground"
                    }
                  >
                    Messages
                  </button>
                  <button
                    type="button"
                    onClick={() => setViewMode("dlq")}
                    className={
                      viewMode === "dlq"
                        ? "rounded-md bg-foreground px-3 py-1.5 text-sm font-medium text-background"
                        : "rounded-md border border-border px-3 py-1.5 text-sm text-muted-foreground hover:text-foreground"
                    }
                  >
                    DLQ
                  </button>
                </div>
                <p className="mt-2 max-w-3xl text-sm text-muted-foreground">
                  {viewMode === "dlq"
                    ? "Dead-lettered jobs for local queues, including failure details and attempt history."
                    : "Recent jobs submitted to local queues, with per-queue status, timing, payload, and result details."}
                </p>
              </div>
              <div className="text-right text-xs text-muted-foreground">
                <div>Window</div>
                <div className="mt-1 text-foreground">{period}</div>
              </div>
            </div>

            <div className="mt-5 grid gap-3 md:grid-cols-4">
              <label className="text-xs text-muted-foreground">
                <span className="mb-2 block uppercase tracking-wide">Queue</span>
                <select
                  value={queueFilter}
                  onChange={(event) => setQueueFilter(event.target.value)}
                  className="w-full rounded-md border border-border bg-sidebar/40 px-3 py-2 text-sm text-foreground outline-none"
                >
                  <option value="">All queues</option>
                  {availableQueues.map((queue) => (
                    <option key={queue} value={queue}>
                      {queue}
                    </option>
                  ))}
                </select>
              </label>
              <label className="text-xs text-muted-foreground">
                <span className="mb-2 block uppercase tracking-wide">Topic</span>
                <select
                  value={topicFilter}
                  onChange={(event) => setTopicFilter(event.target.value)}
                  className="w-full rounded-md border border-border bg-sidebar/40 px-3 py-2 text-sm text-foreground outline-none"
                >
                  <option value="">All topics</option>
                  {availableTopics.map((topic) => (
                    <option key={topic} value={topic}>
                      {topic}
                    </option>
                  ))}
                </select>
              </label>
              <label className="text-xs text-muted-foreground">
                <span className="mb-2 block uppercase tracking-wide">Status</span>
                <select
                  value={statusFilter}
                  onChange={(event) => setStatusFilter(event.target.value)}
                  disabled={viewMode === "dlq"}
                  className="w-full rounded-md border border-border bg-sidebar/40 px-3 py-2 text-sm text-foreground outline-none"
                >
                  <option value="">All statuses</option>
                  <option value="queued">Queued</option>
                  <option value="processing">Processing</option>
                  <option value="retrying">Retrying</option>
                  <option value="completed">Completed</option>
                  <option value="dead_lettered">Dead lettered</option>
                  <option value="cleared">Cleared</option>
                </select>
              </label>
              <div className="flex items-end">
                <div className="w-full rounded-md border border-border bg-sidebar/20 px-3 py-2 text-sm">
                  <span className="text-muted-foreground">Rows</span>
                  <span className="ml-2 font-medium tabular-nums">{formatNumber(messages.length)}</span>
                </div>
              </div>
            </div>

            {messagesError ? <div className="mt-4 text-sm text-red-400">{messagesError}</div> : null}

            <div className="mt-5 overflow-hidden rounded-md border border-border">
              <table className="w-full text-sm">
                <thead className="bg-sidebar/60 text-xs uppercase tracking-wide text-muted-foreground">
                  <tr>
                    <th className="px-4 py-3 text-left font-medium">Queue</th>
                    <th className="px-4 py-3 text-left font-medium">Status</th>
                    <th className="px-4 py-3 text-left font-medium">Inserted</th>
                    <th className="px-4 py-3 text-left font-medium">Picked up</th>
                    <th className="px-4 py-3 text-left font-medium">Finished</th>
                    <th className="px-4 py-3 text-right font-medium">Duration</th>
                    <th className="px-4 py-3 text-left font-medium">Input</th>
                    <th className="px-4 py-3 text-left font-medium">Output</th>
                  </tr>
                </thead>
                <tbody>
                  {messagesLoading && messages.length === 0 ? (
                    <tr>
                      <td colSpan={8} className="px-4 py-8 text-center text-muted-foreground">
                        Loading queue messages…
                      </td>
                    </tr>
                  ) : messages.length === 0 ? (
                    <tr>
                      <td colSpan={8} className="px-4 py-8 text-center text-muted-foreground">
                        {viewMode === "dlq"
                          ? "No dead-lettered queue messages found for the selected filters and timeframe."
                          : "No queue messages found for the selected filters and timeframe."}
                      </td>
                    </tr>
                  ) : (
                    messages.map((message) => (
                      <PubSubMessageRow
                        key={messageRowKey(message)}
                        message={message}
                        open={expandedRow === messageRowKey(message)}
                        attempts={attemptsByRow[messageRowKey(message)] ?? []}
                        onToggle={() =>
                          setExpandedRow((current) => (current === messageRowKey(message) ? null : messageRowKey(message)))
                        }
                      />
                    ))
                  )}
                </tbody>
              </table>
            </div>
          </section>
        </div>
      </div>
    </div>
  );
}

interface ChartPoint {
  time: number;
  queued: number;
  active: number;
}

function queueID(topicName: string, subscriptionName: string) {
  return `${topicName}\u0000${subscriptionName}`;
}

function findSubscription(topics: PubSubTopic[], id: string): PubSubSubscription | null {
  for (const topic of topics) {
    for (const subscription of topic.subscriptions) {
      if (queueID(topic.name, subscription.name) === id) {
        return subscription;
      }
    }
  }
  return null;
}

function QueueSparkline({
  points,
  label,
  period,
}: {
  points: ChartPoint[];
  label: string;
  period: (typeof chartPeriods)[number]["value"];
}) {
  const [hover, setHover] = useState<ChartPoint | null>(null);
  const width = 900;
  const height = 160;
  const padding = { top: 18, right: 34, bottom: 14, left: 6 };
  const plotWidth = width - padding.left - padding.right;
  const plotHeight = height - padding.top - padding.bottom;
  const now = Date.now();
  const start = now - periodToMs(period);
  const end = now;
  const domain = normalizeChartDomain(points, start, end);
  const maxValue = Math.max(1, ...domain.map((point) => point.queued));
  const activeMax = Math.max(1, ...domain.map((point) => point.active));
  const span = Math.max(1, end - start);
  const x = (time: number) => padding.left + ((time - start) / span) * plotWidth;
  const y = (value: number) => padding.top + plotHeight - (value / maxValue) * plotHeight;
  const yActive = (value: number) => padding.top + plotHeight - (value / activeMax) * plotHeight;
  const queuedPath = linePath(domain, (point) => x(point.time), (point) => y(point.queued));
  const activePath = linePath(domain, (point) => x(point.time), (point) => yActive(point.active));
  const latestPoint = domain.at(-1) ?? null;
  const legendPoint = hover ?? latestPoint;
  const tooltipX = hover ? Math.min(width - 170, Math.max(12, x(hover.time) + 12)) : 0;
  const tooltipY = hover ? Math.max(12, y(hover.queued) - 66) : 0;

  if (domain.length === 0) {
    return (
      <div className="flex h-36 items-center justify-center text-xs text-muted-foreground">
        No history yet for {label}.
      </div>
    );
  }

  return (
    <div className="relative">
      <svg
        viewBox={`0 0 ${width} ${height}`}
        className="h-40 w-full"
        role="img"
        aria-label={`${label} queue throughput`}
        onPointerMove={(event) => {
          const rect = event.currentTarget.getBoundingClientRect();
          const pointerX = ((event.clientX - rect.left) / rect.width) * width;
          const targetTime = start + ((pointerX - padding.left) / plotWidth) * span;
          setHover(nearestPoint(domain, targetTime));
        }}
        onPointerLeave={() => setHover(null)}
      >
        <defs>
          <linearGradient id={`pulse-${safeID(label)}`} x1="0" x2="0" y1="0" y2="1">
            <stop offset="0%" stopColor="#fbbf24" stopOpacity="0.22" />
            <stop offset="100%" stopColor="#fbbf24" stopOpacity="0" />
          </linearGradient>
        </defs>
        <path
          d={`${queuedPath} L ${x(end).toFixed(2)} ${height - padding.bottom} L ${x(start).toFixed(2)} ${height - padding.bottom} Z`}
          fill={`url(#pulse-${safeID(label)})`}
        />
        <path d={queuedPath} fill="none" stroke="#fbbf24" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round" />
        <path d={activePath} fill="none" stroke="#bef264" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round" />
        <line
          x1={width - padding.right + 8}
          x2={width - padding.right + 8}
          y1={padding.top}
          y2={height - padding.bottom}
          stroke="#bef264"
          strokeOpacity="0.45"
        />
        <text x={width - padding.right + 14} y={padding.top + 4} fill="#fff" className="text-[10px]">
          {formatNumber(activeMax)}
        </text>
        <text x={width - padding.right + 14} y={height - padding.bottom} fill="#fff" className="text-[10px]">
          0
        </text>
        <g transform="translate(10 14)">
          <LegendItem x={0} color="#fbbf24" label="Queued" value={formatNumber(legendPoint?.queued ?? 0)} />
          <LegendItem x={118} color="#bef264" label="Active" value={formatNumber(legendPoint?.active ?? 0)} />
        </g>
        <text x={padding.left} y={height - 2} fill="#fff" className="text-[10px]">
          {formatTime(start)}
        </text>
        <text x={width - padding.right} y={height - 2} textAnchor="end" fill="#fff" className="text-[10px]">
          {formatTime(end)}
        </text>
        {hover ? (
          <>
            <line
              x1={x(hover.time)}
              x2={x(hover.time)}
              y1={padding.top}
              y2={height - padding.bottom}
              stroke="#ffffff"
              strokeOpacity="0.18"
              strokeDasharray="4 5"
            />
            <circle cx={x(hover.time)} cy={y(hover.queued)} r="5" fill="#fbbf24" />
            <circle cx={x(hover.time)} cy={y(hover.queued)} r="10" fill="#fbbf24" opacity="0.16" />
            <circle cx={x(hover.time)} cy={yActive(hover.active)} r="5" fill="#bef264" />
            <g transform={`translate(${tooltipX} ${tooltipY})`}>
              <rect width="158" height="60" rx="8" fill="#111" stroke="#333" />
              <text x="10" y="18" fill="#f5f5f5" className="text-[11px] font-medium">
                {formatTime(hover.time)}
              </text>
              <TooltipRow y={36} color="#fbbf24" label="Queued" value={formatNumber(hover.queued)} />
              <TooltipRow y={52} color="#bef264" label="Active" value={formatNumber(hover.active)} />
            </g>
          </>
        ) : null}
      </svg>
    </div>
  );
}

function TooltipRow({ y, color, label, value }: { y: number; color: string; label: string; value: string }) {
  return (
    <>
      <circle cx="12" cy={y - 3} r="3" fill={color} />
      <text x="22" y={y} fill="#fff" className="text-[10px]">
        {label}
      </text>
      <text x="148" y={y} fill="#f5f5f5" textAnchor="end" className="text-[10px] font-medium">
        {value}
      </text>
    </>
  );
}

function LegendItem({ x, color, label, value }: { x: number; color: string; label: string; value: string }) {
  return (
    <g transform={`translate(${x} 0)`}>
      <circle cx="0" cy="0" r="3.5" fill={color} />
      <text x="9" y="4" fill="#fff" className="text-[10px]">
        {label}
      </text>
      <text x="72" y="4" fill="#f5f5f5" className="text-[10px] font-medium">
        {value}
      </text>
    </g>
  );
}

function TopicCard({
  topic,
  history,
  latest,
  period,
}: {
  topic: PubSubTopic;
  history: PubSubHistoryPoint[];
  latest: PubSubSnapshot | null;
  period: (typeof chartPeriods)[number]["value"];
}) {
  const failed = topic.subscriptions.reduce((sum, item) => sum + item.failed, 0);
  const inFlight = topic.subscriptions.reduce((sum, item) => sum + item.in_flight, 0);

  return (
    <section className="rounded-md border border-border p-6">
      <div className="flex flex-wrap items-start justify-between gap-4">
        <div>
          <h2 className="text-base font-medium">{topic.name}</h2>
          <div className="mt-2 flex flex-wrap gap-2 text-xs text-muted-foreground">
            {topic.subject ? <code>{topic.subject}</code> : null}
            {topic.stream ? <code>{topic.stream}</code> : null}
            <span>{formatDelivery(topic.delivery)}</span>
            {topic.ordering_key ? <span>ordering: {topic.ordering_key}</span> : null}
          </div>
        </div>
        <div className="grid grid-cols-3 gap-3 text-right">
          <MiniStat label="Queued" value={formatNumber(topic.pending)} />
          <MiniStat label="Active" value={formatNumber(inFlight)} tone={inFlight > 0 ? "live" : undefined} />
          <MiniStat label="Failures" value={formatNumber(failed)} tone={failed > 0 ? "error" : undefined} />
        </div>
      </div>

      {topic.subscriptions.length > 0 ? (
        <div className={topic.subscriptions.length === 1 ? "mt-8" : "mt-8 grid gap-4 lg:grid-cols-2"}>
          {topic.subscriptions.map((sub) => (
            <QueueSparkline
              key={sub.name}
              label={`${topic.name} / ${sub.name}`}
              points={buildChartPoints(history, latest, queueID(topic.name, sub.name), period)}
              period={period}
            />
          ))}
        </div>
      ) : (
        <div className="mt-8 h-40" />
      )}

      <div className="mt-5 overflow-hidden rounded-md border border-border">
        <table className="w-full text-sm">
          <thead className="bg-sidebar/60 text-xs uppercase tracking-wide text-muted-foreground">
            <tr>
              <th className="px-4 py-3 text-left font-medium">Subscription</th>
              <th className="px-4 py-3 text-right font-medium">Queued</th>
              <th className="px-4 py-3 text-right font-medium">Picked up</th>
              <th className="px-4 py-3 text-right font-medium">In flight</th>
              <th className="px-4 py-3 text-right font-medium">Workers</th>
              <th className="px-4 py-3 text-right font-medium">Avg duration</th>
              <th className="px-4 py-3 text-right font-medium">Failures</th>
            </tr>
          </thead>
          <tbody>
            {topic.subscriptions.length === 0 ? (
              <tr>
                <td colSpan={7} className="px-4 py-4 text-muted-foreground">
                  No subscriptions registered for this topic.
                </td>
              </tr>
            ) : (
              topic.subscriptions.map((sub) => <SubscriptionRow key={sub.name} sub={sub} />)
            )}
          </tbody>
        </table>
      </div>

      {failed > 0 ? (
        <p className="mt-3 text-xs text-red-500">
          {failed} failed processing attempt{failed === 1 ? "" : "s"} recorded for this topic.
        </p>
      ) : null}
    </section>
  );
}

function SubscriptionRow({ sub }: { sub: PubSubSubscription }) {
  const failureCount = sub.failed + sub.dead_lettered + sub.redelivered;
  return (
    <tr className="border-t border-border">
      <td className="px-4 py-3">
        <div className="font-medium">{sub.name}</div>
        <div className="mt-1 text-xs text-muted-foreground">
          {sub.service_name || "package handler"} · ack deadline {formatMillis(sub.ack_deadline_ms ?? 0)}
        </div>
      </td>
      <td className="px-4 py-3 text-right tabular-nums">{formatNumber(sub.pending)}</td>
      <td className="px-4 py-3 text-right tabular-nums">{formatNumber(sub.picked_up)}</td>
      <td className="px-4 py-3 text-right tabular-nums">
        <span className={sub.in_flight > 0 ? "text-lime-300" : ""}>{formatNumber(sub.in_flight)}</span>
      </td>
      <td className="px-4 py-3 text-right tabular-nums">{formatWorkers(sub)}</td>
      <td className="px-4 py-3 text-right tabular-nums">{formatMillis(sub.avg_duration_ms)}</td>
      <td className="px-4 py-3 text-right tabular-nums">
        <span className={failureCount > 0 ? "text-red-500" : "text-muted-foreground"}>
          {formatNumber(failureCount)}
        </span>
      </td>
    </tr>
  );
}

function PubSubMessageRow({
  message,
  open,
  attempts,
  onToggle,
}: {
  message: PubSubMessage;
  open: boolean;
  attempts: PubSubMessageAttempt[];
  onToggle: () => void;
}) {
  return (
    <>
      <tr className="border-t border-border align-top">
        <td className="px-4 py-3">
          <button type="button" onClick={onToggle} className="w-full text-left">
            <div className="font-medium">{message.subscription_name}</div>
            <div className="mt-1 text-xs text-muted-foreground">
              {message.topic_name}
              {message.service_name ? ` · ${message.service_name}` : ""}
            </div>
            <div className="mt-1 font-mono text-[11px] text-muted-foreground">{message.message_id}</div>
          </button>
        </td>
        <td className="px-4 py-3">
          <StatusBadge status={message.status} />
          <div className="mt-2 flex flex-wrap items-center gap-2 text-xs text-muted-foreground">
            {message.attempt && message.attempt > 0 ? <span>attempt {message.attempt}</span> : null}
            {message.trace_id ? (
              <Link
                to="/$appId/envs/local/traces/$traceId"
                params={{ appId: message.app_id, traceId: message.trace_id }}
                className="text-sky-300 hover:text-sky-200"
              >
                Trace
              </Link>
            ) : null}
          </div>
        </td>
        <td className="px-4 py-3 text-xs text-muted-foreground">{formatTimestamp(message.inserted_at)}</td>
        <td className="px-4 py-3 text-xs text-muted-foreground">{formatTimestampValue(message.picked_up_at)}</td>
        <td className="px-4 py-3 text-xs text-muted-foreground">{formatTimestampValue(message.finished_at)}</td>
        <td className="px-4 py-3 text-right tabular-nums">{formatMillis(message.duration_ms ?? 0)}</td>
        <td className="px-4 py-3">
          <JSONPreview value={message.payload} label="Input" />
        </td>
        <td className="px-4 py-3">
          <JSONPreview value={message.result} error={message.error} label="Output" />
        </td>
      </tr>
      {open ? (
        <tr className="border-t border-border/60 bg-sidebar/20">
          <td colSpan={8} className="px-4 py-4">
            <div className="space-y-3">
              <div className="text-xs uppercase tracking-wide text-muted-foreground">Attempt history</div>
              <div className="overflow-hidden rounded-md border border-border">
                <table className="w-full text-sm">
                  <thead className="bg-sidebar/50 text-xs uppercase tracking-wide text-muted-foreground">
                    <tr>
                      <th className="px-3 py-2 text-left font-medium">Attempt</th>
                      <th className="px-3 py-2 text-left font-medium">Status</th>
                      <th className="px-3 py-2 text-left font-medium">Picked up</th>
                      <th className="px-3 py-2 text-left font-medium">Finished</th>
                      <th className="px-3 py-2 text-right font-medium">Duration</th>
                      <th className="px-3 py-2 text-left font-medium">Trace</th>
                    </tr>
                  </thead>
                  <tbody>
                    {attempts.length === 0 ? (
                      <tr>
                        <td colSpan={6} className="px-3 py-4 text-muted-foreground">
                          No attempt history loaded yet.
                        </td>
                      </tr>
                    ) : (
                      attempts.map((attempt) => (
                        <tr key={`${attempt.message_id}-${attempt.subscription_name}-${attempt.attempt}`} className="border-t border-border">
                          <td className="px-3 py-2 tabular-nums">{attempt.attempt}</td>
                          <td className="px-3 py-2">
                            <StatusBadge status={attempt.status} />
                          </td>
                          <td className="px-3 py-2 text-xs text-muted-foreground">{formatTimestampValue(attempt.picked_up_at)}</td>
                          <td className="px-3 py-2 text-xs text-muted-foreground">{formatTimestampValue(attempt.finished_at)}</td>
                          <td className="px-3 py-2 text-right tabular-nums">{formatMillis(attempt.duration_ms ?? 0)}</td>
                          <td className="px-3 py-2 text-xs">
                            {attempt.trace_id ? (
                              <Link
                                to="/$appId/envs/local/traces/$traceId"
                                params={{ appId: attempt.app_id, traceId: attempt.trace_id }}
                                className="text-sky-300 hover:text-sky-200"
                              >
                                Open trace
                              </Link>
                            ) : (
                              <span className="text-muted-foreground">—</span>
                            )}
                          </td>
                        </tr>
                      ))
                    )}
                  </tbody>
                </table>
              </div>
            </div>
          </td>
        </tr>
      ) : null}
    </>
  );
}

function StatusBadge({ status }: { status: string }) {
  return (
    <span
      className={cn(
        "inline-flex rounded-full border px-2 py-1 text-[11px] font-medium uppercase tracking-wide",
        status === "completed" && "border-lime-900 bg-lime-950/40 text-lime-300",
        status === "queued" && "border-amber-900 bg-amber-950/30 text-amber-300",
        status === "processing" && "border-sky-900 bg-sky-950/30 text-sky-300",
        status === "retrying" && "border-orange-900 bg-orange-950/30 text-orange-300",
        status === "dead_lettered" && "border-red-900 bg-red-950/30 text-red-300",
        status === "cleared" && "border-border bg-sidebar/50 text-muted-foreground",
      )}
    >
      {statusLabel(status)}
    </span>
  );
}

function JSONPreview({ value, error, label }: { value: unknown; error?: string; label: string }) {
  const text = previewJSON(value);
  const hasValue = text !== "null" && text !== "{}" && text !== "[]";
  const detailsText = hasValue ? text : error || "No data";
  return (
    <details className="group max-w-[320px]">
      <summary className="cursor-pointer list-none text-xs text-muted-foreground hover:text-foreground">
        <span className="font-medium text-foreground">{label}</span>
        <span className="ml-2 truncate">{truncateSingleLine(detailsText, 56)}</span>
      </summary>
      <pre className="mt-3 max-h-56 overflow-auto rounded-md border border-border bg-background/70 p-3 text-[11px] leading-5 text-muted-foreground whitespace-pre-wrap break-all">
        {detailsText}
      </pre>
    </details>
  );
}

function InlineStat({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex items-baseline gap-2 whitespace-nowrap">
      <span className="text-xs uppercase tracking-wide text-muted-foreground">{label}</span>
      <span className="text-base font-medium tabular-nums">{value}</span>
    </div>
  );
}

function MiniStat({ label, value, tone }: { label: string; value: string; tone?: "live" | "error" }) {
  return (
    <div>
      <div className="text-xs text-muted-foreground">{label}</div>
      <div
        className={
          tone === "live"
            ? "mt-1 font-medium text-lime-300 tabular-nums"
            : tone === "error"
              ? "mt-1 font-medium text-red-400 tabular-nums"
              : "mt-1 font-medium tabular-nums"
        }
      >
        {value}
      </div>
    </div>
  );
}

function summarize(topics: PubSubTopic[]) {
  let subscriptions = 0;
  let pending = 0;
  let pickedUp = 0;
  let inFlight = 0;
  let totalDuration = 0;
  for (const topic of topics) {
    pending += topic.pending;
    subscriptions += topic.subscriptions.length;
    for (const sub of topic.subscriptions) {
      pickedUp += sub.picked_up;
      inFlight += sub.in_flight;
      totalDuration += sub.avg_duration_ms * sub.picked_up;
    }
  }
  return {
    subscriptions,
    pending,
    pickedUp,
    inFlight,
    avgDurationMs: pickedUp > 0 ? totalDuration / pickedUp : 0,
  };
}

function buildChartPoints(
  history: PubSubHistoryPoint[],
  latest: PubSubSnapshot | null,
  queue: string,
  period: (typeof chartPeriods)[number]["value"],
): ChartPoint[] {
  if (!queue) {
    return [];
  }
  const cutoff = Date.now() - periodToMs(period);
  const raw = [...history];
  if (latest?.updated_at) {
    const latestTime = new Date(latest.updated_at).getTime();
    const hasLatest = raw.some((point) => point.updated_at && new Date(point.updated_at).getTime() === latestTime);
    if (!hasLatest) {
      raw.push({ topics: latest.topics, updated_at: latest.updated_at });
    }
  }
  const sorted = raw
    .filter((point) => point.updated_at)
    .filter((point) => new Date(point.updated_at ?? 0).getTime() >= cutoff)
    .sort((a, b) => new Date(a.updated_at ?? 0).getTime() - new Date(b.updated_at ?? 0).getTime());
  return sorted.map((point) => {
    const time = new Date(point.updated_at ?? 0).getTime();
    const sub = findSubscription(point.topics ?? [], queue);
    return {
      time,
      queued: sub?.pending ?? 0,
      active: sub?.in_flight ?? 0,
    };
  });
}

function mergeLiveSnapshot(current: PubSubSnapshot | null, next: PubSubSnapshot): PubSubSnapshot {
  if (!current || current.app_id !== next.app_id) {
    return next;
  }
  const history = [...(current.history ?? [])];
  if (next.updated_at) {
    const nextTime = new Date(next.updated_at).getTime();
    const existing = history.findIndex((point) => point.updated_at && new Date(point.updated_at).getTime() === nextTime);
    const point = { topics: next.topics, updated_at: next.updated_at };
    if (existing >= 0) {
      history[existing] = point;
    } else {
      history.push(point);
    }
  }
  return {
    ...next,
    history,
  };
}

function periodToMs(period: (typeof chartPeriods)[number]["value"]) {
  switch (period) {
    case "5m":
      return 5 * 60 * 1000;
    case "1h":
      return 60 * 60 * 1000;
    case "6h":
      return 6 * 60 * 60 * 1000;
    case "24h":
      return 24 * 60 * 60 * 1000;
    case "15m":
    default:
      return 15 * 60 * 1000;
  }
}

function normalizeChartDomain(points: ChartPoint[], start: number, end: number) {
  const visible = points.filter((point) => point.time >= start && point.time <= end);
  if (visible.length === 0) {
    return [];
  }
  const first = visible[0];
  const last = visible.at(-1) ?? first;
  const domain = [...visible];
  if (first.time > start) {
    domain.unshift({
      ...first,
      time: start,
    });
  }
  if (last.time < end) {
    domain.push({
      ...last,
      time: end,
    });
  }
  return domain;
}

function linePath<T>(items: T[], getX: (item: T) => number, getY: (item: T) => number) {
  return items.map((item, index) => `${index === 0 ? "M" : "L"} ${getX(item).toFixed(2)} ${getY(item).toFixed(2)}`).join(" ");
}

function nearestPoint(points: ChartPoint[], targetTime: number): ChartPoint | null {
  let best: ChartPoint | null = null;
  let bestDistance = Number.POSITIVE_INFINITY;
  for (const point of points) {
    const distance = Math.abs(point.time - targetTime);
    if (distance < bestDistance) {
      best = point;
      bestDistance = distance;
    }
  }
  return best;
}

function formatNumber(value: number) {
  return Math.round(value || 0).toLocaleString();
}

function upsertMessage(current: PubSubMessage[], next: PubSubMessage) {
  const key = messageRowKey(next);
  const index = current.findIndex((item) => messageRowKey(item) === key);
  if (index < 0) {
    return [next, ...current].sort(compareMessages);
  }
  const copy = current.slice();
  copy[index] = next;
  copy.sort(compareMessages);
  return copy;
}

function upsertAttempt(current: PubSubMessageAttempt[], next: PubSubMessageAttempt) {
  const index = current.findIndex((item) => item.attempt === next.attempt);
  if (index < 0) {
    return [next, ...current].sort((a, b) => b.attempt - a.attempt);
  }
  const copy = current.slice();
  copy[index] = next;
  copy.sort((a, b) => b.attempt - a.attempt);
  return copy;
}

function messageToAttempt(message: PubSubMessage): PubSubMessageAttempt {
  return {
    app_id: message.app_id,
    message_id: message.message_id,
    topic_name: message.topic_name,
    subscription_name: message.subscription_name,
    service_name: message.service_name,
    status: message.status,
    trace_id: message.trace_id,
    attempt: message.attempt ?? Math.max(1, message.deliveries || 1),
    payload: message.payload,
    result: message.result,
    error: message.error,
    deliveries: message.deliveries,
    inserted_at: message.inserted_at,
    picked_up_at: message.picked_up_at,
    finished_at: message.finished_at,
    duration_ms: message.duration_ms,
  };
}

function messageWithinFilters(
  message: PubSubMessage,
  period: (typeof chartPeriods)[number]["value"],
  topicFilter: string,
  queueFilter: string,
  statusFilter: string,
) {
  const cutoff = Date.now() - periodToMs(period);
  const inserted = Date.parse(message.inserted_at || "");
  if (Number.isFinite(inserted) && inserted < cutoff) {
    return false;
  }
  if (topicFilter && message.topic_name !== topicFilter) {
    return false;
  }
  if (queueFilter && message.subscription_name !== queueFilter) {
    return false;
  }
  if (statusFilter && statusFilter !== "all" && message.status !== statusFilter) {
    return false;
  }
  return true;
}

function compareMessages(a: PubSubMessage, b: PubSubMessage) {
  return Date.parse(b.inserted_at || "") - Date.parse(a.inserted_at || "");
}

function messageRowKey(message: PubSubMessage) {
  return `${message.message_id}\u0000${message.subscription_name}`;
}

function formatMillis(value: number) {
  if (!value) {
    return "0ms";
  }
  if (value < 1000) {
    return `${Math.round(value)}ms`;
  }
  return `${(value / 1000).toFixed(2)}s`;
}

function formatTime(value: number) {
  return new Date(value).toLocaleTimeString([], { hour: "numeric", minute: "2-digit" });
}

function formatTimestampValue(value?: string) {
  return value ? formatTimestamp(value) : "—";
}

function safeID(value: string) {
  return value.replace(/[^a-zA-Z0-9_-]/g, "-");
}

function formatWorkers(sub: PubSubSubscription) {
  if (!sub.max_workers || sub.max_workers <= 0) {
    return `${sub.in_flight}/unbounded`;
  }
  return `${sub.in_flight}/${sub.max_workers}`;
}

function formatDelivery(value?: string) {
  switch (value) {
    case "at_least_once":
      return "at least once";
    case "exactly_once":
      return "exactly once";
    default:
      return value || "delivery unknown";
  }
}

function statusLabel(value: string) {
  switch (value) {
    case "dead_lettered":
      return "Dead lettered";
    default:
      return value.replace(/_/g, " ");
  }
}

function previewJSON(value: unknown) {
  if (value === undefined) {
    return "null";
  }
  try {
    return JSON.stringify(value, null, 2);
  } catch {
    return String(value);
  }
}

function truncateSingleLine(value: string, limit: number) {
  const line = value.replace(/\s+/g, " ").trim();
  if (line.length <= limit) {
    return line;
  }
  return `${line.slice(0, Math.max(0, limit - 1))}\u2026`;
}
