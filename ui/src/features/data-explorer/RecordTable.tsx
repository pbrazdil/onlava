import { Card, CardContent } from "@/components/primitives/Card";
import { formatJSON } from "@/lib/utils";
import { recordColumns, type DataObjectSummary, type DataRecord } from "./dataExplorerClient";

export function RecordTable({
  object,
  records,
  loading,
  error,
}: {
  object: DataObjectSummary | null;
  records: DataRecord[];
  loading: boolean;
  error: string | null;
}) {
  const columns = recordColumns(object, records);
  if (error) {
    return (
      <div className="flex h-full items-center justify-center p-6">
        <Card>
          <CardContent>
            <p className="text-sm text-red-400">{error}</p>
          </CardContent>
        </Card>
      </div>
    );
  }
  if (!object) {
    return (
      <div className="flex h-full items-center justify-center p-6">
        <p className="text-sm text-muted-foreground">Select a tenant and object to query records.</p>
      </div>
    );
  }
  if (loading) {
    return (
      <div className="flex h-full items-center justify-center p-6">
        <p className="text-sm text-muted-foreground">Querying records...</p>
      </div>
    );
  }
  if (records.length === 0) {
    return (
      <div className="flex h-full items-center justify-center p-6">
        <p className="text-sm text-muted-foreground">No records matched this query.</p>
      </div>
    );
  }
  return (
    <div className="h-full min-h-0 overflow-auto">
      <table className="min-w-full text-sm">
        <thead className="sticky top-0 bg-muted">
          <tr>
            {columns.map((column) => (
              <th key={column} className="px-3 py-2 text-left text-xs font-medium uppercase text-muted-foreground">
                {column}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {records.map((record, rowIndex) => (
            <tr key={String(record.id ?? rowIndex)} className="border-t border-border">
              {columns.map((column) => (
                <td key={`${rowIndex}-${column}`} className="px-3 py-2 align-top">
                  <code className="whitespace-pre-wrap break-all text-xs">{renderCell(record[column])}</code>
                </td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

function renderCell(value: unknown): string {
  if (value === null || value === undefined) {
    return "null";
  }
  if (typeof value === "string") {
    return value;
  }
  return formatJSON(value);
}
