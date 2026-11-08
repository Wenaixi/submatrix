// API Client for SubMatrix Go Core
export function getApiKey(): string {
  return localStorage.getItem("submatrix_api_key") || localStorage.getItem("apiKey") || "wenxi-admin";
}

export function setApiKey(key: string) {
  localStorage.setItem("submatrix_api_key", key);
  localStorage.setItem("apiKey", key);
}

export async function apiRequest<T = any>(endpoint: string, options: RequestInit = {}): Promise<T> {
  const token = getApiKey();
  const headers = new Headers(options.headers || {});
  if (!headers.has("X-API-Key")) {
    headers.set("X-API-Key", token);
  }
  if (!headers.has("Content-Type") && options.body && typeof options.body === "string") {
    headers.set("Content-Type", "application/json");
  }

  const separator = endpoint.includes("?") ? "&" : "?";
  const url = `${endpoint}${separator}token=${encodeURIComponent(token)}`;

  const res = await fetch(url, { ...options, headers });
  if (res.status === 401) {
    throw new Error("UNAUTHORIZED");
  }
  if (!res.ok) {
    let errText = "";
    try {
      const errJson = await res.json();
      errText = errJson.error || errJson.message || res.statusText;
    } catch {
      errText = await res.text();
    }
    throw new Error(errText || `HTTP ${res.status}`);
  }
  return res.json();
}

// 1. 状态遥测
export interface TelemetryStatus {
  checking: boolean;
  fetching: boolean;
  stepName: string;
  proxyCount: number;
  processed: number;
  available: number;
  progress: number;
  eta: number;
  lastCheck?: {
    time: string;
    duration: number;
    total: number;
    available: number;
    traffic: number;
  };
  subStorePort: string;
  subStorePath: string;
}

export const getStatus = () => apiRequest<TelemetryStatus>("/api/status");
export const triggerCheck = () => apiRequest<{ message: string }>("/api/trigger-check", { method: "POST" });
export const forceClose = () => apiRequest<{ message: string }>("/api/force-close", { method: "POST" });

// 2. 分组矩阵
export interface GroupItem {
  Name: string;
  DisplayName?: string;
  Tag?: string;
  Enabled?: boolean;
  CheckAlive?: boolean | null;
  CheckSpeed?: boolean | null;
  MinSpeed?: number | null;
  MediaCheck?: boolean | null;
  TagIPType?: boolean | null;
  RenameNode?: boolean | null;
  AppendOriginalName?: boolean | null;
  DropOnFail?: boolean | null;
  OverwriteURL?: string;
  Timeout?: number;
  URLs?: string[];
  Proxy?: string;
  FilterProtocols?: string[];
  RulePreset?: string;
  SuccessLimit?: number;
  DownloadMB?: number;
  // 健全容错兼容字段
  name?: string;
  displayName?: string;
  display_name?: string;
  tag?: string;
  enabled?: boolean;
  check_alive?: boolean;
  checkAlive?: boolean;
  check_speed?: boolean;
  checkSpeed?: boolean;
  min_speed?: number;
  minSpeed?: number;
  media_check?: boolean;
  mediaCheck?: boolean;
  tag_ip_type?: boolean;
  tagIPType?: boolean;
  rename_node?: boolean;
  renameNode?: boolean;
  append_original_name?: boolean;
  appendOriginalName?: boolean;
  drop_on_fail?: boolean;
  dropOnFail?: boolean;
  overwrite_url?: string;
  overwriteUrl?: string;
  timeout?: number;
  urls?: string[];
  proxy?: string;
  filter_protocols?: string[];
  filterProtocols?: string[];
  rule_preset?: string;
  rulePreset?: string;
  success_limit?: number;
  successLimit?: number;
  download_mb?: number;
  downloadMB?: number;
  priority?: number;
  Priority?: number;
}

export interface GroupsResponse {
  groups: GroupItem[];
  apiKey: string;
  listenPort: string;
}

export const getGroups = () => apiRequest<GroupsResponse>("/api/groups");
export const saveGroups = (groups: GroupItem[]) => apiRequest("/api/groups", {
  method: "POST",
  body: JSON.stringify({ groups })
});
export const toggleGroup = (name: string, enabled: boolean) => apiRequest("/api/groups/toggle", {
  method: "POST",
  body: JSON.stringify({ name, enabled })
});

// 3. 运行日志
export interface LogsResponse {
  logs: string[];
}
export const getLogs = () => apiRequest<LogsResponse>("/api/logs");
export const clearLogs = () => apiRequest("/api/logs/clear", { method: "POST" });

// 4. 文件服务
export interface FileItem {
  name: string;
  size: number;
  size_human: string;
  updated: string;
  path: string;
  raw_url: string;
}

export interface FilesResponse {
  files: FileItem[];
  output_dir: string;
}
export const getFilesData = () => apiRequest<FilesResponse>("/api/files-data");

// 5. 节点质检分析报告全量结构
export interface CheckInfo {
  check_time?: string;
  check_duration?: string;
  check_duration_raw?: number;
  check_count?: string;
  check_count_raw?: number;
  check_traffic_total?: string;
  check_traffic_total_raw?: number;
  check_traffic_download_raw?: number;
  check_traffic_upload_raw?: number;
  total_subs_size?: string;
  check_min_speed?: number;
}

export interface SubRankingItem {
  url: string;
  avg_speed?: string;
  file_size?: string;
  traffic?: string;
  stats?: {
    total: number;
    success: number;
    rate: string;
  };
  locations?: Record<string, number>;
  protocols?: Record<string, number>;
  top_locations?: string[];
}

export interface ReportData {
  summary?: string;
  check_info?: CheckInfo;
  global_analysis?: {
    alive_count?: number;
    geography_distribution?: Record<string, number>;
    protocol_distribution?: Record<string, number>;
    quality_metrics?: Record<string, any>;
  };
  subs_ranking?: SubRankingItem[];
  subs_ranking_bad?: SubRankingItem[];
}

export interface AnalysisReportResponse {
  report?: string;
  report_data?: ReportData;
}
export const getAnalysisReport = () => apiRequest<AnalysisReportResponse>("/api/analysis-report");

// 6. 配置中心
export interface ConfigResponse {
  content: string;
  sub_store_path?: string;
}
export const getConfig = () => apiRequest<ConfigResponse>("/api/config");
export const saveConfig = (content: string) => apiRequest("/api/config", {
  method: "POST",
  body: JSON.stringify({ content })
});

// 7. 分享管理
export interface ShareItem {
  id: string;
  group: string;
  enabled: boolean;
  note: string;
  created_at: string;
  expires_at: string;
  files: string[];
  total_hits: number;
  unique_ips: number;
  last_hit: string;
  per_file: Record<string, number>;
  link: string;
}

export interface SharesResponse {
  shares: ShareItem[];
}

export interface ShareIPHit {
  ip: string;
  time: string;
  file: string;
  ua: string;
}

export interface ShareStatsResponse {
  share_id: string;
  total_hits: number;
  unique_ips: number;
  last_hit: string;
  per_file: Record<string, number>;
  ip_history: ShareIPHit[];
}

export const getShares = () => apiRequest<SharesResponse>("/api/shares");
export const createShare = (payload: { group: string; note?: string; expires_at?: string; files?: string[] }) =>
  apiRequest("/api/shares", { method: "POST", body: JSON.stringify(payload) });
export const toggleShare = (id: string, enabled: boolean) =>
  apiRequest("/api/shares/toggle", { method: "POST", body: JSON.stringify({ id, enabled }) });
export const deleteShare = (id: string) =>
  apiRequest("/api/shares/delete", { method: "POST", body: JSON.stringify({ id }) });
export const getShareStats = (code: string) =>
  apiRequest<ShareStatsResponse>(`/api/shares/stats?code=${encodeURIComponent(code)}`);
