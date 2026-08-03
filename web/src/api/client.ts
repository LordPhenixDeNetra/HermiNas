// Thin fetch wrapper for HermiNas' Go API (engine/api, M1.5). Same-origin
// in production (Go serves the built SPA and the API from one port, see
// vite.config.ts's dev proxy for the equivalent during `npm run dev`), so
// every path here is relative rather than an absolute base URL.

export class ApiError extends Error {
  status: number;

  constructor(status: number, message: string) {
    super(message);
    this.status = status;
    this.name = "ApiError";
  }
}

async function request<T>(path: string, options: RequestInit = {}, token?: string): Promise<T> {
  const headers: Record<string, string> = {
    "Content-Type": "application/json",
    ...(options.headers as Record<string, string> | undefined),
  };
  if (token) {
    headers.Authorization = `Bearer ${token}`;
  }

  const resp = await fetch(path, { ...options, headers });
  const text = await resp.text();
  const data = text ? JSON.parse(text) : undefined;

  if (!resp.ok) {
    const message = (data && typeof data === "object" && "error" in data && String(data.error)) || resp.statusText;
    throw new ApiError(resp.status, message);
  }
  return data as T;
}

export interface LoginResponse {
  token: string;
  role: string;
}

export function login(username: string, password: string): Promise<LoginResponse> {
  return request<LoginResponse>("/api/v1/auth/login", {
    method: "POST",
    body: JSON.stringify({ username, password }),
  });
}

export interface QueryResult {
  row_count: number;
  cached: boolean;
  rows: Record<string, unknown>[];
}

export function runQuery(sql: string, token: string): Promise<QueryResult> {
  return request<QueryResult>("/api/v1/query", { method: "POST", body: JSON.stringify({ sql }) }, token);
}

export interface DatasetColumn {
  name: string;
  type: string;
  nullable: boolean;
}

export interface Dataset {
  name: string;
  columns: DatasetColumn[];
  order_by: string[] | null;
  partition_by_column: string;
  ttl_days: number;
  version: number;
  created_at: string;
  updated_at: string;
}

export function listDatasets(token: string): Promise<Dataset[]> {
  return request<Dataset[]>("/api/v1/datasets", { method: "GET" }, token);
}
