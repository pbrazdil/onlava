import { Button } from "@/components/primitives/Button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/primitives/Card";
import { Select } from "@/components/primitives/Input";
import type { DataInspectResponse, DataObjectSummary } from "./dataExplorerClient";

export function ObjectList({
  data,
  selectedTenantKey,
  selectedObjectName,
  onTenantChange,
  onObjectChange,
}: {
  data: DataInspectResponse | null;
  selectedTenantKey: string;
  selectedObjectName: string;
  onTenantChange: (tenant: string) => void;
  onObjectChange: (object: string) => void;
}) {
  const tenantObjects = (data?.objects ?? []).filter((object) => object.tenant_key === selectedTenantKey);
  return (
    <div className="flex h-full min-h-0 flex-col gap-3 p-3">
      <Card>
        <CardHeader>
          <CardTitle>Tenants</CardTitle>
        </CardHeader>
        <CardContent className="space-y-3">
          <Select
            aria-label="Data tenant"
            value={selectedTenantKey}
            onChange={(event) => onTenantChange(event.target.value)}
          >
            {(data?.tenants ?? []).map((tenant) => (
              <option key={tenant.key} value={tenant.key}>
                {tenant.name || tenant.key}
              </option>
            ))}
          </Select>
          <div className="grid grid-cols-2 gap-2 text-xs text-muted-foreground">
            <Metric label="Tenants" value={data?.tenants.length ?? 0} />
            <Metric label="Objects" value={tenantObjects.length} />
          </div>
        </CardContent>
      </Card>

      <Card className="min-h-0 flex-1">
        <CardHeader>
          <CardTitle>Objects</CardTitle>
        </CardHeader>
        <CardContent className="min-h-0 space-y-2 overflow-auto">
          {tenantObjects.map((object) => (
            <ObjectButton
              key={object.id}
              object={object}
              selected={object.name === selectedObjectName}
              onClick={() => onObjectChange(object.name)}
            />
          ))}
          {tenantObjects.length === 0 ? (
            <p className="text-sm text-muted-foreground">No dynamic objects for this tenant.</p>
          ) : null}
        </CardContent>
      </Card>
    </div>
  );
}

function ObjectButton({
  object,
  selected,
  onClick,
}: {
  object: DataObjectSummary;
  selected: boolean;
  onClick: () => void;
}) {
  return (
    <Button
      tone={selected ? "primary" : "secondary"}
      className="h-auto w-full justify-start px-3 py-2 text-left"
      onClick={onClick}
    >
      <span className="min-w-0">
        <span className="block truncate">{object.name}</span>
        <span className="block truncate text-xs opacity-70">{object.physical_table}</span>
      </span>
    </Button>
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
