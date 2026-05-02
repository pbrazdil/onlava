import { Link } from "@tanstack/react-router";
import { useEffect, useMemo, useRef, useState } from "react";
import {
  createStoredRequest,
  deleteStoredRequest,
  fetchStoredRequests,
  updateStoredRequest,
} from "../lib/graphql";
import { useDashboard } from "../lib/dashboard-context";
import {
  closeExplorerTab,
  ensureExplorerTabs,
  loadPersistedTabs,
  makeTabFromEndpoint,
  makeTabFromStoredRequest,
  normalizeActiveTab,
  persistTabs,
  reconcileTabsWithEndpoints,
  type RequestTab,
} from "../lib/api-explorer";
import {
  cn,
  formatDurationNanos,
  formatTime,
  materializePath,
  parseJSONInput,
  processOutputText,
  renderMetadataPath,
  tryParseJSON,
} from "../lib/utils";
import {
  CompactRequestEditor,
  RequestFolder,
  RequestLogs,
  ResponsePanel,
  SourceLinkButton,
  TabStrip,
  baseName,
  defaultRequestPayloadText,
  endpointHasPathParams,
  endpointHasRequestPayload,
  isPlaceholderPayload,
  lineMatchesField,
  renderResponseBody,
} from "./requests-editor";
import {
  DeleteStoredRequestModal,
  EditStoredRequestModal,
  EndpointSelector,
  IconActivity,
  IconPanelLeft,
  IconTraceRequest,
  StoreRequestModal,
  openEditor,
} from "./requests-modals";
import type {
  ApiCallResponse,
  APIEncodingRPC,
  EndpointOption,
  StoredRequest,
  StoredRequestInput,
  ServiceRPC,
} from "../lib/types";

const REQUESTS_SIDEBAR_STORAGE_KEY = "onlava:requests-sidebar-collapsed";
const REQUESTS_SIDEBAR_WIDTH = 280;

export function RequestsPage() {
  const { appId, apiEncoding, callAPI, meta, outputs, refreshAll, rpc, status, traces } = useDashboard();
  const [items, setItems] = useState<StoredRequest[]>([]);
  const [requestError, setRequestError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [search, setSearch] = useState("");
  const [tabs, setTabs] = useState<RequestTab[]>([]);
  const [activeTabID, setActiveTabID] = useState<string | null>(null);
  const [traceServiceFilter, setTraceServiceFilter] = useState("");
  const [traceEndpointFilter, setTraceEndpointFilter] = useState("");
  const [traceStatusFilter, setTraceStatusFilter] = useState<"all" | "ok" | "error">("all");
  const [traceIDFilter, setTraceIDFilter] = useState("");
  const [showEndpointPicker, setShowEndpointPicker] = useState(false);
  const [showStoreModal, setShowStoreModal] = useState(false);
  const [editingRequest, setEditingRequest] = useState<StoredRequest | null>(null);
  const [deletingRequest, setDeletingRequest] = useState<StoredRequest | null>(null);
  const [menuRequestID, setMenuRequestID] = useState<string | null>(null);
  const [folderOpen, setFolderOpen] = useState<Record<string, boolean>>({
    my: true,
    shared: true,
  });
  const [sidebarCollapsed, setSidebarCollapsed] = useState(false);
  const [requestSeq, setRequestSeq] = useState(1);

  const endpointOptions = useMemo<EndpointOption[]>(() => {
    const combined = new Map<string, EndpointOption>();
    for (const svc of apiEncoding?.services ?? []) {
      for (const rpc of svc.rpcs) {
        const key = `${svc.name}.${rpc.name}`;
        combined.set(key, {
          key,
          svcName: svc.name,
          rpcName: rpc.name,
          method: rpc.methods?.[0] || "GET",
          path: rpc.path || `/${svc.name}.${rpc.name}`,
          accessType: rpc.access_type,
        });
      }
    }
    for (const svc of meta?.svcs ?? []) {
      for (const rpc of svc.rpcs) {
        const key = `${svc.name}.${rpc.name}`;
        const current = combined.get(key);
        combined.set(key, {
          key,
          svcName: svc.name,
          rpcName: rpc.name,
          method: current?.method || rpc.http_methods?.[0] || "GET",
          path: renderMetadataPath(rpc.path) || current?.path || `/${svc.name}.${rpc.name}`,
          accessType: current?.accessType || rpc.access_type,
        });
      }
    }
    const all = Array.from(combined.values());
    return all
      .slice()
      .sort((a, b) => a.svcName.localeCompare(b.svcName) || a.rpcName.localeCompare(b.rpcName));
  }, [apiEncoding, meta?.svcs]);

  const endpointMap = useMemo(
    () => new Map(endpointOptions.map((item) => [item.key, item])),
    [endpointOptions],
  );
  const endpointMetaMap = useMemo(() => {
    const entries = new Map<string, ServiceRPC>();
    for (const svc of meta?.svcs ?? []) {
      for (const rpc of svc.rpcs) {
        entries.set(`${svc.name}.${rpc.name}`, rpc);
      }
    }
    return entries;
  }, [meta?.svcs]);

  useEffect(() => {
    const persisted = loadPersistedTabs(appId);
    setTabs(persisted.tabs);
    setActiveTabID(persisted.activeTabID);
  }, [appId]);

  useEffect(() => {
    if (typeof window === "undefined") {
      return;
    }
    const raw = window.localStorage.getItem(REQUESTS_SIDEBAR_STORAGE_KEY);
    setSidebarCollapsed(raw === "1");
  }, []);

  useEffect(() => {
    if (typeof window === "undefined") {
      return;
    }
    window.localStorage.setItem(REQUESTS_SIDEBAR_STORAGE_KEY, sidebarCollapsed ? "1" : "0");
  }, [sidebarCollapsed]);

  useEffect(() => {
    if (endpointOptions.length === 0) {
      return;
    }
    setTabs((current) => {
      const nextTabs = ensureExplorerTabs(
        reconcileTabsWithEndpoints(current, endpointMap),
        endpointOptions,
      );
      setActiveTabID((currentActiveTabID) => normalizeActiveTab(currentActiveTabID, nextTabs));
      return nextTabs;
    });
  }, [endpointMap, endpointOptions]);

  useEffect(() => {
    if (tabs.length === 0) {
      return;
    }
    const nextActive = normalizeActiveTab(activeTabID, tabs);
    if (nextActive !== activeTabID) {
      setActiveTabID(nextActive);
      return;
    }
    persistTabs(appId, nextActive, tabs);
  }, [activeTabID, appId, tabs]);

  const refreshRequests = async () => {
    setLoading(true);
    try {
      const next = await fetchStoredRequests(appId);
      setItems(next);
      setRequestError(null);
    } catch (err) {
      setRequestError(err instanceof Error ? err.message : String(err));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    void refreshRequests();
  }, [appId]);

  useEffect(() => {
    const onKeyDown = (event: KeyboardEvent) => {
      if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === "k") {
        event.preventDefault();
        setShowEndpointPicker(true);
        return;
      }
      if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === "b") {
        event.preventDefault();
        setSidebarCollapsed((current) => !current);
      }
    };
    window.addEventListener("keydown", onKeyDown);
    return () => window.removeEventListener("keydown", onKeyDown);
  }, []);

  useEffect(() => {
    const onPointerDown = (event: PointerEvent) => {
      const target = event.target as HTMLElement | null;
      if (!target?.closest("[data-request-menu]")) {
        setMenuRequestID(null);
      }
    };
    window.addEventListener("pointerdown", onPointerDown);
    return () => window.removeEventListener("pointerdown", onPointerDown);
  }, []);

  const myRequests = items.filter((item) => !item.shared);
  const sharedRequests = items.filter((item) => item.shared);
  const activeTab = tabs.find((tab) => tab.id === activeTabID) || null;
  const activeEndpoint = useMemo(() => {
    if (!activeTab) {
      return null;
    }
    return endpointMap.get(`${activeTab.svcName}.${activeTab.rpcName}`) ?? null;
  }, [activeTab, endpointMap]);
  const recentLogLines = useMemo(() => {
    const correlationID = activeTab?.correlationID?.trim();
    if (!correlationID) {
      return [];
    }
    const scoped = outputs.flatMap((item) =>
      processOutputText(item)
        .split("\n")
        .map((line) => line.trim())
        .filter(Boolean)
        .filter((line) => lineMatchesField(line, "x_correlation_id", correlationID))
        .map((line) => ({
          created_at: item.created_at,
          pid: item.pid,
          stream: item.stream,
          line,
        })),
    );
    return scoped.slice(-250);
  }, [activeTab?.correlationID, outputs]);
  const activeEndpointMeta = useMemo<ServiceRPC | null>(() => {
    if (!activeTab) {
      return null;
    }
    return endpointMetaMap.get(`${activeTab.svcName}.${activeTab.rpcName}`) ?? null;
  }, [activeTab, endpointMetaMap]);
  const activeHasPathParams = useMemo(
    () => endpointHasPathParams(activeEndpointMeta, activeEndpoint?.path),
    [activeEndpoint?.path, activeEndpointMeta],
  );
  const activeHasRequestPayload = useMemo(
    () => endpointHasRequestPayload(activeEndpointMeta),
    [activeEndpointMeta],
  );
  const activeDefaultPayloadText = useMemo(
    () => defaultRequestPayloadText(activeEndpointMeta?.request_schema),
    [activeEndpointMeta?.request_schema],
  );
  const endpointEditorTarget = useMemo(() => {
    const loc = activeEndpointMeta?.loc;
    const root = status?.appRoot;
    if (!loc?.filename || !root) {
      return null;
    }
    const parts = [root];
    if (loc.pkg_path) {
      parts.push(loc.pkg_path);
    }
    parts.push(loc.filename);
    return {
      file: parts.join("/"),
      line: loc.src_line_start ?? 0,
      col: loc.src_col_start ?? 0,
      label: `${baseName(loc.filename)}${loc.src_line_start ? `:${loc.src_line_start}` : ""}`,
    };
  }, [activeEndpointMeta?.loc, status?.appRoot]);
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
        if (traceStatusFilter === "ok" && trace.is_error) {
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
  const activeTrace = useMemo(
    () =>
      activeTab?.response?.trace_id
        ? traces.find((trace) => trace.trace_id === activeTab.response?.trace_id) ?? null
        : null,
    [activeTab?.response?.trace_id, traces],
  );

  useEffect(() => {
    if (!activeTab || !activeHasRequestPayload || !activeDefaultPayloadText) {
      return;
    }
    if (!isPlaceholderPayload(activeTab.payloadText)) {
      return;
    }
    updateTab(activeTab.id, { payloadText: activeDefaultPayloadText });
  }, [activeDefaultPayloadText, activeHasRequestPayload, activeTab]);

  function updateTab(tabID: string, patch: Partial<RequestTab>) {
    setTabs((current) => current.map((tab) => (tab.id === tabID ? { ...tab, ...patch } : tab)));
  }

  function openStoredRequest(item: StoredRequest) {
    const endpoint = endpointMap.get(`${item.svcName}.${item.rpcName}`);
    const next = makeTabFromStoredRequest(item, endpoint);
    setTabs((current) => [...current, next]);
    setActiveTabID(next.id);
  }

  function applyEndpointToTab(tabID: string, endpoint: EndpointOption) {
    const endpointMeta = endpointMetaMap.get(`${endpoint.svcName}.${endpoint.rpcName}`) ?? null;
    setTabs((current) =>
      current.map((tab) =>
        tab.id === tabID
          ? {
              ...tab,
              title: tab.storedRequestID ? tab.title : `${endpoint.svcName}.${endpoint.rpcName}`,
              svcName: endpoint.svcName,
              rpcName: endpoint.rpcName,
              method: endpoint.method,
              path: endpoint.path,
              pathParamsText: "[]",
              payloadText: defaultRequestPayloadText(endpointMeta?.request_schema),
              response: null,
              responseError: null,
            }
          : tab,
      ),
    );
  }

  async function removeStoredRequest(item: StoredRequest) {
    await deleteStoredRequest(appId, item.id);
    setTabs((current) =>
      current.map((tab) =>
        tab.storedRequestID === item.id ? { ...tab, storedRequestID: undefined, shared: false } : tab,
      ),
    );
    await refreshRequests();
  }

  async function persistStoredRequest(
    tab: RequestTab,
    params:
      | { mode: "new"; title: string; shared: boolean }
      | { mode: "update"; storedRequestID: string },
  ) {
    const input: StoredRequestInput = {
      title: params.mode === "new" ? params.title : tab.title,
      rpcName: tab.rpcName,
      svcName: tab.svcName,
      shared: params.mode === "new" ? params.shared : tab.shared,
      data: {
        method: tab.method,
        pathParams: parseJSONInput(tab.pathParamsText),
        payload: parseJSONInput(tab.payloadText),
      },
    };

    let storedRequestID = tab.storedRequestID;
    let nextTitle = tab.title;
    let nextShared = tab.shared;
    if (params.mode === "update") {
      await updateStoredRequest(appId, params.storedRequestID, input);
      storedRequestID = params.storedRequestID;
    } else {
      storedRequestID = await createStoredRequest(appId, input);
      nextTitle = params.title;
      nextShared = params.shared;
    }

    updateTab(tab.id, {
      storedRequestID,
      title: nextTitle,
      shared: nextShared,
      responseError: null,
    });
    await refreshRequests();
  }

  async function updateStoredRequestMeta(item: StoredRequest, patch: { title: string; shared: boolean }) {
    await updateStoredRequest(appId, item.id, {
      title: patch.title,
      rpcName: item.rpcName,
      svcName: item.svcName,
      shared: patch.shared,
      data: item.data,
    });
    setTabs((current) =>
      current.map((tab) =>
        tab.storedRequestID === item.id ? { ...tab, title: patch.title, shared: patch.shared } : tab,
      ),
    );
    await refreshRequests();
  }

  return (
    <section className="w-full h-[calc(100vh-(var(--header-height)))] grid grid-cols-3">
      <div className="col-span-2 overflow-hidden border-border border-r min-w-0">
        <div className="relative flex max-w-full" style={{ height: "calc(100vh - var(--header-height))" }}>
          <div
            aria-hidden="true"
            className="relative shrink-0 bg-transparent transition-[width] duration-200 ease-linear"
            style={{ width: sidebarCollapsed ? 0 : REQUESTS_SIDEBAR_WIDTH }}
          />
          <aside
            className={cn(
              "absolute inset-y-0 left-0 z-10 overflow-auto border-border border-r bg-sidebar transition-[left] duration-200 ease-linear",
            )}
            style={{
              width: REQUESTS_SIDEBAR_WIDTH,
              left: sidebarCollapsed ? -REQUESTS_SIDEBAR_WIDTH : 0,
            }}
          >
            <div className="px-2 pt-4 pb-2">
              <input
                className="h-8 w-full rounded-md border border-border px-3 text-sm shadow-none"
                placeholder="Search"
                value={search}
                onChange={(event) => setSearch(event.target.value)}
              />
            </div>
            {requestError ? (
              <div className="px-4 py-3 text-sm text-red-500">{requestError}</div>
            ) : null}
            {loading ? (
              <div className="flex w-full pt-6 flex-1 items-start justify-center">
                <div className="h-6 w-6 animate-spin rounded-full border-2 border-border border-t-foreground" />
              </div>
            ) : (
              <div className="px-2 pb-4 space-y-4">
                <RequestFolder
                  items={myRequests}
                  label="My requests"
                  menuRequestID={menuRequestID}
                  open={folderOpen.my ?? true}
                  onDelete={(item) => setDeletingRequest(item)}
                  onEdit={(item) => setEditingRequest(item)}
                  onOpen={openStoredRequest}
                  onToggleMenu={(itemID) =>
                    setMenuRequestID((current) => (current === itemID ? null : itemID))
                  }
                  onToggleOpen={() =>
                    setFolderOpen((current) => ({ ...current, my: !(current.my ?? true) }))
                  }
                  searchQuery={search}
                />
                <RequestFolder
                  items={sharedRequests}
                  label="Shared requests"
                  menuRequestID={menuRequestID}
                  open={folderOpen.shared ?? true}
                  onDelete={(item) => setDeletingRequest(item)}
                  onEdit={(item) => setEditingRequest(item)}
                  onOpen={openStoredRequest}
                  onToggleMenu={(itemID) =>
                    setMenuRequestID((current) => (current === itemID ? null : itemID))
                  }
                  onToggleOpen={() =>
                    setFolderOpen((current) => ({ ...current, shared: !(current.shared ?? true) }))
                  }
                  searchQuery={search}
                />
              </div>
            )}
          </aside>

          <div className="flex flex-col flex-1 w-full overflow-hidden">
            <header className="flex h-12 shrink-0 items-center gap-2 bg-sidebar border-b border-border transition-[width,height] ease-linear">
              <div className="flex items-center gap-2 px-4">
                <button
                  type="button"
                  className="-ml-1 inline-flex h-8 w-8 items-center justify-center rounded-md transition-colors hover:bg-accent hover:text-accent-foreground"
                  onClick={() => setSidebarCollapsed((current) => !current)}
                >
                  <IconPanelLeft className="h-4 w-4" />
                </button>
                <div className="mr-2 h-4 w-px bg-border" />
                <div className="text-sm">API Explorer</div>
              </div>
            </header>

            <div className="w-full bg-muted transition-colors ease-linear border-t-0">
              <TabStrip
                activeTabID={activeTabID}
                tabs={tabs}
                onActivate={setActiveTabID}
                onClose={(tabID) => {
                  setTabs((current) => {
                    const next = closeExplorerTab(current, activeTabID, tabID, endpointOptions);
                    setActiveTabID(next.activeTabID);
                    return next.tabs;
                  });
                }}
                onNew={() => {
                  if (endpointOptions.length === 0) {
                    return;
                  }
                  const next = makeTabFromEndpoint(endpointOptions[0]);
                  setTabs((current) => [...current, next]);
                  setActiveTabID(next.id);
                }}
              />
            </div>

            <div className="flex-1 min-w-0 flex flex-col overflow-auto">
              {activeTab ? (
                <div className="p-4 w-full min-w-0 max-w-full">
                  <div className="space-y-5">
                    <div>
                      <EndpointSelector
                        currentKey={`${activeTab.svcName}.${activeTab.rpcName}`}
                        endpoints={endpointOptions}
                        invalidEndpoint={!activeEndpoint}
                        open={showEndpointPicker}
                        onClose={() => setShowEndpointPicker(false)}
                        onOpen={() => setShowEndpointPicker(true)}
                        onSelect={(endpoint) => applyEndpointToTab(activeTab.id, endpoint)}
                      />
                      {endpointEditorTarget ? (
                        <SourceLinkButton
                          label={endpointEditorTarget.label}
                          onClick={() =>
                            void openEditor(
                              appId,
                              rpc,
                              endpointEditorTarget.file,
                              endpointEditorTarget.line,
                              endpointEditorTarget.col,
                            )
                          }
                        />
                      ) : null}
                    </div>

                    <CompactRequestEditor
                      disabled={!status?.running}
                      hasAuth={!!meta?.auth_handler}
                      hasPathParams={activeHasPathParams}
                      hasRequestPayload={activeHasRequestPayload}
                      requestTab={activeTab}
                      onCall={() => void callCurrentTab(activeTab)}
                      onOpenStoreModal={() => setShowStoreModal(true)}
                      onResetPath={() => {
                        const endpoint = endpointMap.get(`${activeTab.svcName}.${activeTab.rpcName}`);
                        if (!endpoint) {
                          return;
                        }
                        updateTab(activeTab.id, {
                          method: endpoint.method,
                          path: endpoint.path,
                          pathParamsText: "[]",
                        });
                      }}
                      onUpdate={(patch) => updateTab(activeTab.id, patch)}
                      onUpdatePathParams={(value) => {
                        updateTab(activeTab.id, { pathParamsText: value });
                        const endpoint = endpointMap.get(`${activeTab.svcName}.${activeTab.rpcName}`);
                        if (!endpoint) {
                          return;
                        }
                        try {
                          updateTab(activeTab.id, {
                            path: materializePath(endpoint.path, parseJSONInput(value)),
                          });
                        } catch {
                          // Leave current path as-is while editing incomplete JSON.
                        }
                      }}
                    />

                    {activeTab.responseError ? (
                      <div className="rounded-md border border-red-500/30 bg-red-500/10 px-4 py-3 text-sm text-red-500">
                        {activeTab.responseError}
                      </div>
                    ) : null}

                    {activeTab.response ? (
                      <ResponsePanel
                        appId={appId}
                        response={activeTab.response}
                        traceDuration={activeTrace ? formatDurationNanos(activeTrace.duration_nanos) : ""}
                      />
                    ) : null}

                    {recentLogLines.length > 0 ? <RequestLogs lines={recentLogLines} /> : null}
                  </div>
                </div>
              ) : null}
            </div>
          </div>
        </div>
      </div>

      <div className="col-span-1 overflow-auto">
        <div className="overflow-y-auto overflow-x-hidden" style={{ height: "calc(100vh - var(--header-height))" }}>
          <section>
            <div className="flex items-center justify-between pt-4 px-4 bg-sidebar pb-2">
              <div className="flex items-center gap-2 -mt-2">
                <IconActivity className="h-4 w-4" />
                <p className="text-sm">Traces</p>
              </div>
              <button
                type="button"
                className="rounded-md border border-border px-2 py-1 text-xs transition-colors hover:bg-sidebar-accent hover:text-sidebar-accent-foreground disabled:opacity-50"
                disabled={traces.length === 0}
                onClick={() => void rpc?.request("traces/clear", { app_id: appId }).then(() => refreshAll())}
              >
                Clear traces
              </button>
            </div>
            <div className="pb-3 bg-sidebar px-4 pt-0 border-b border-border">
              <div className="flex flex-col gap-1.5 mb-3">
                <div className="flex items-start gap-2.5">
                  <div className="flex-1 flex flex-col gap-0.5">
                    <span className="text-xs font-medium uppercase tracking-wide text-muted-foreground">Type</span>
                    <select className="h-9 w-full rounded-md border border-border px-3 text-sm" value="api-calls" disabled>
                      <option value="api-calls">API Calls</option>
                    </select>
                  </div>
                </div>
              </div>
              <div className="space-y-3 devdash-trace-filters">
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

                <div className="text-xs font-medium uppercase tracking-wide text-muted-foreground">Trace ID</div>
                <input
                  className="h-9 w-full rounded-md border border-border px-3 text-sm"
                  placeholder="Trace ID"
                  value={traceIDFilter}
                  onChange={(event) => setTraceIDFilter(event.target.value)}
                />
              </div>
            </div>
            <div className="px-4 py-3">
              {filteredTraces.slice(0, 50).length === 0 ? (
                <p className="text-sm text-muted-foreground">No traces match the current filters.</p>
              ) : (
                <div className="w-full">
                  {filteredTraces.slice(0, 50).map((trace) => (
                    <Link
                      key={`${trace.trace_id}/${trace.span_id}`}
                      to="/$appId/envs/local/traces/$traceId"
                      params={{ appId, traceId: trace.trace_id }}
                      className="group/traceRow block border-b border-border text-sm transition-colors hover:bg-accent/50"
                    >
                      <div className="relative flex h-12 items-center justify-between px-2 py-2">
                        <div className="min-w-0 flex items-center h-full space-x-2">
                          <figure
                            className={cn(
                              "h-3 w-3 rounded-full",
                              trace.is_error ? "bg-red-500" : "bg-success",
                            )}
                          />
                          <div className="text-sm min-w-0 shrink flex items-start flex-col justify-start">
                            <div className="text-sm min-w-0 flex items-center">
                              <div className="flex-none w-5">
                                <IconTraceRequest className="h-4 w-4 inline-block mr-2" />
                              </div>
                              <div className="shrink truncate">
                                {trace.service_name || "unknown service"}.{trace.endpoint_name || trace.type}
                              </div>
                            </div>
                            <div className="mt-1 text-xs text-muted-foreground font-mono truncate">
                              {trace.trace_id}
                            </div>
                          </div>
                        </div>
                        <div className="min-w-0 flex flex-col text-right text-xs mt-1 text-muted-foreground">
                          <span>{formatDurationNanos(trace.duration_nanos)}</span>
                          <span>{formatTime(trace.started_at)}</span>
                        </div>
                      </div>
                    </Link>
                  ))}
                </div>
              )}
            </div>
          </section>
        </div>
      </div>
      {activeTab ? (
        <StoreRequestModal
          items={items}
          open={showStoreModal}
          requestTab={activeTab}
          onClose={() => setShowStoreModal(false)}
          onSave={(params) => persistStoredRequest(activeTab, params)}
        />
      ) : null}
      <EditStoredRequestModal
        item={editingRequest}
        items={items}
        onClose={() => setEditingRequest(null)}
        onSave={(item, patch) => updateStoredRequestMeta(item, patch)}
      />
      <DeleteStoredRequestModal
        item={deletingRequest}
        onClose={() => setDeletingRequest(null)}
        onDelete={removeStoredRequest}
      />
    </section>
  );

  async function callCurrentTab(tab: RequestTab) {
    const correlationID = `dash-call-${requestSeq}`;
    setRequestSeq((current) => current + 1);
    updateTab(tab.id, { correlationID, response: null, responseError: null });
    try {
      const result = await callAPI({
        service: tab.svcName,
        endpoint: tab.rpcName,
        path: tab.path,
        method: tab.method,
        payload: tryParseJSON(tab.payloadText),
        authToken: tab.authToken,
        correlationID,
      });
      updateTab(tab.id, { correlationID, response: result, responseError: null });
    } catch (err) {
      updateTab(tab.id, {
        correlationID,
        response: null,
        responseError: err instanceof Error ? err.message : String(err),
      });
    }
  }
}
