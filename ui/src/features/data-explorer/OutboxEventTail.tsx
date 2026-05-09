import { Card, CardContent, CardHeader, CardTitle } from "@/components/primitives/Card";
import { formatJSON, formatTimestamp } from "@/lib/utils";
import type { DataOutboxEvent, DataOutboxSummary } from "./dataExplorerClient";

export function OutboxEventTail({
  events,
  summary,
  loading,
}: {
  events: DataOutboxEvent[];
  summary: DataOutboxSummary | null;
  loading: boolean;
}) {
  return (
    <div className="h-full min-h-0 p-3">
      <Card className="flex h-full min-h-0 flex-col">
        <CardHeader>
          <CardTitle>Outbox</CardTitle>
        </CardHeader>
        <CardContent className="min-h-0 flex-1 space-y-3 overflow-auto">
          <div className="grid grid-cols-2 gap-2 text-xs">
            <Metric label="Latest seq" value={summary?.latest_seq ?? 0} />
            <Metric label="Unpublished" value={summary?.unpublished ?? 0} />
          </div>
          {loading ? <p className="text-sm text-muted-foreground">Loading events...</p> : null}
          {events.map((event) => (
            <details key={event.seq} className="rounded-md border border-border p-3 text-sm">
              <summary className="cursor-pointer">
                <span className="font-medium">#{event.seq}</span> {event.action}
                <span className="ml-2 text-xs text-muted-foreground">{formatTimestamp(event.created_at)}</span>
              </summary>
              <pre className="mt-3 overflow-auto text-xs leading-6">{formatJSON(event)}</pre>
            </details>
          ))}
          {!loading && events.length === 0 ? (
            <p className="text-sm text-muted-foreground">No outbox events for this selection.</p>
          ) : null}
        </CardContent>
      </Card>
    </div>
  );
}

function Metric({ label, value }: { label: string; value: number }) {
  return (
    <div>
      <div className="text-muted-foreground">{label}</div>
      <div className="text-sm font-medium text-foreground">{value}</div>
    </div>
  );
}
