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
  // 容错兼容小写
  name?: string;
  displayName?: string;
  tag?: string;
  enabled?: boolean;
  urls?: string[];
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

// 5. 节点分析
export interface AnalysisReportResponse {
  total: number;
  available: number;
  nodes?: any[];
  countries?: Record<string, number>;
  protocols?: Record<string, number>;
  speed_tiers?: Record<string, number>;
  avg_speed?: number;
  max_speed?: number;
  generated_at?: string;
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
