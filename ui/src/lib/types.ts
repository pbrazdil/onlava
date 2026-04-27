export interface AppSummary {
  id: string;
  name: string;
  app_root: string;
  offline: boolean;
}

export interface AppStatus {
  running: boolean;
  appID: string;
  appRoot: string;
  pid?: string;
  meta?: DashboardMeta;
  addr?: string;
  apiEncoding?: APIEncoding;
  compiling: boolean;
  compileError?: string;
}

export interface DashboardMeta {
  module_path?: string;
  svcs: ServiceMeta[];
  cron_jobs: CronJob[];
  middleware: MiddlewareMeta[];
  pubsub_topics?: unknown[];
  sql_databases: DatabaseMeta[];
  auth_handler?: AuthHandlerMeta;
}

export interface APIEncoding {
  services: APIEncodingService[];
}

export interface APIEncodingService {
  name: string;
  rpcs: APIEncodingRPC[];
}

export interface APIEncodingRPC {
  name: string;
  path: string;
  methods: string[];
  raw: boolean;
  access_type: string;
  service_name: string;
}

export interface ServiceMeta {
  name: string;
  rel_path: string;
  rpcs: ServiceRPC[];
}

export interface ServiceRPC {
  name: string;
  access_type: string;
  proto: string;
  wire?: WireInfo;
  path: MetadataPath;
  loc?: SourceLoc;
  http_methods: string[];
  request_schema?: unknown;
  response_schema?: unknown;
}

export interface WireInfo {
  available: boolean;
  unsupported_reason?: string;
  schema_hash?: string;
  path?: string;
}

export interface SourceLoc {
  pkg_path?: string;
  pkg_name?: string;
  filename?: string;
  start_pos?: number;
  end_pos?: number;
  src_line_start?: number;
  src_line_end?: number;
  src_col_start?: number;
  src_col_end?: number;
}

export interface MetadataPath {
  type: string;
  segments: MetadataPathSegment[];
}

export interface MetadataPathSegment {
  type: "LITERAL" | "PARAM";
  value: string;
  value_type: string;
}

export interface MiddlewareMeta {
  name: {
    pkg: string;
    name: string;
  };
  global: boolean;
  service_name?: string;
  target?: unknown;
}

export interface AuthHandlerMeta {
  name: string;
  pkg_path: string;
  pkg_name: string;
  auth_data?: unknown;
  params?: unknown;
}

export interface CronJob {
  id: string;
  title: string;
  schedule?: string;
  every?: string;
  endpoint?: {
    service_name?: string;
    rpc_name?: string;
  };
}

export interface DatabaseMeta {
  name: string;
}

export interface TraceSummary {
  trace_id: string;
  span_id: string;
  type: string;
  is_root: boolean;
  is_error: boolean;
  started_at: string;
  duration_nanos: number;
  service_name?: string;
  endpoint_name?: string | null;
  message_id?: string | null;
  parent_span_id?: string | null;
}

export interface ProcessOutput {
  appID: string;
  pid: string;
  stream: string;
  output: string;
  created_at: string;
}

export interface PubSubSnapshot {
  app_id: string;
  topics: PubSubTopic[];
  updated_at?: string;
  period?: string;
  history?: PubSubHistoryPoint[];
}

export interface PubSubMessagesResponse {
  app_id: string;
  period?: string;
  topic_name?: string;
  queue_name?: string;
  status?: string;
  messages: PubSubMessage[];
}

export interface PubSubHistoryPoint {
  topics: PubSubTopic[];
  updated_at?: string;
}

export interface PubSubMessage {
  app_id: string;
  message_id: string;
  topic_name: string;
  subscription_name: string;
  service_name?: string;
  status: string;
  trace_id?: string;
  attempt?: number;
  payload?: unknown;
  result?: unknown;
  error?: string;
  deliveries: number;
  inserted_at: string;
  picked_up_at?: string;
  finished_at?: string;
  duration_ms?: number;
}

export interface PubSubMessageAttempt {
  app_id: string;
  message_id: string;
  topic_name: string;
  subscription_name: string;
  service_name?: string;
  status: string;
  trace_id?: string;
  attempt: number;
  payload?: unknown;
  result?: unknown;
  error?: string;
  deliveries: number;
  inserted_at: string;
  picked_up_at?: string;
  finished_at?: string;
  duration_ms?: number;
}

export interface PubSubTopic {
  name: string;
  stream?: string;
  subject?: string;
  delivery?: string;
  ordering_key?: string;
  published: number;
  pending: number;
  subscriptions: PubSubSubscription[];
}

export interface PubSubSubscription {
  name: string;
  service_name?: string;
  max_workers: number;
  max_ack_pending?: number;
  ack_deadline_ms?: number;
  message_retention_s?: number;
  pending: number;
  ack_pending: number;
  redelivered: number;
  picked_up: number;
  completed: number;
  failed: number;
  dead_lettered: number;
  in_flight: number;
  avg_duration_ms: number;
}

export interface DashboardNotification {
  method: string;
  params: unknown;
}

export interface StoredRequest {
  id: string;
  title: string;
  rpcName: string;
  svcName: string;
  shared: boolean;
  data: StoredRequestData;
}

export interface StoredRequestData {
  method: string;
  pathParams: unknown;
  payload: unknown;
}

export interface StoredRequestInput {
  title: string;
  rpcName: string;
  svcName: string;
  shared: boolean;
  data: {
    method: string;
    pathParams: unknown;
    payload: unknown;
  };
}

export interface ApiCallResponse {
  status: string;
  status_code: number;
  body: string;
  trace_id?: string;
}

export interface EndpointOption {
  key: string;
  svcName: string;
  rpcName: string;
  method: string;
  path: string;
  accessType?: string;
}
