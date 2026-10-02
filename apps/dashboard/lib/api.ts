const API_URL = process.env.NEXT_PUBLIC_API_URL || "http://localhost:5000";
const API_KEY = process.env.NEXT_PUBLIC_API_KEY || "";

export interface Project {
  id: string;
  workspace_id: string;
  name: string;
  tracking_id: string;
  allowed_domains: string[];
  timezone: string;
  retention_days: number;
  enabled: boolean;
  created_at: string;
  updated_at: string;
}

export interface AnalyticsSummary {
  total_page_views: number;
  unique_visitors: number;
  sessions: number;
  bounce_rate: number;
  avg_session_duration: number;
}

export interface TrafficData {
  date: string;
  page_views: number;
  visitors: number;
}

export interface TopPage {
  path: string;
  views: number;
  unique_visitors: number;
}

export interface TrafficSource {
  source: string;
  category: string;
  page_views: number;
  visitors: number;
  percentage: number;
}

export interface CountryStats {
  country: string;
  page_views: number;
  visitors: number;
  percentage: number;
}

export interface DeviceBreakdown {
  name: string;
  page_views: number;
  visitors: number;
  percentage: number;
}

export interface DevicesStats {
  device_types: DeviceBreakdown[];
  browsers: DeviceBreakdown[];
  operating_systems: DeviceBreakdown[];
}

async function fetchAPI<T>(path: string, options?: RequestInit): Promise<T> {
  const headers: Record<string, string> = { "Content-Type": "application/json", ...options?.headers as Record<string, string> };
  if (API_KEY && !headers["Authorization"]) {
    headers["Authorization"] = `Bearer ${API_KEY}`;
  }
  const res = await fetch(`${API_URL}${path}`, {
    ...options,
    headers,
  });
  if (!res.ok) {
    const body = await res.json().catch(() => ({}));
    throw new Error(body.error || `Request failed: ${res.status}`);
  }
  return res.json();
}

export function getProjects(): Promise<Project[]> {
  return fetchAPI<Project[]>("/v1/analytics/projects");
}

function withProject(path: string, projectId?: string): string {
  if (!projectId) return path;
  const separator = path.includes("?") ? "&" : "?";
  return `${path}${separator}project_id=${encodeURIComponent(projectId)}`;
}

export function getSummary(projectId?: string): Promise<AnalyticsSummary> {
  return fetchAPI<AnalyticsSummary>(withProject("/v1/analytics/summary", projectId));
}

export function getTraffic(days = 30, projectId?: string): Promise<TrafficData[]> {
  return fetchAPI<TrafficData[]>(withProject(`/v1/analytics/traffic?days=${days}`, projectId));
}

export function getTopPages(limit = 20, projectId?: string): Promise<TopPage[]> {
  return fetchAPI<TopPage[]>(withProject(`/v1/analytics/pages?limit=${limit}`, projectId));
}

export function getSources(projectId?: string): Promise<TrafficSource[]> {
  return fetchAPI<TrafficSource[]>(withProject("/v1/analytics/sources", projectId));
}

export function getCountries(projectId?: string): Promise<CountryStats[]> {
  return fetchAPI<CountryStats[]>(withProject("/v1/analytics/countries", projectId));
}

export function getDevices(projectId?: string): Promise<DevicesStats> {
  return fetchAPI<DevicesStats>(withProject("/v1/analytics/devices", projectId));
}

export interface RealtimePage {
  path: string;
  visitors: number;
}

export interface RealtimeStats {
  active_visitors: number;
  pages: RealtimePage[];
}

export function getRealtime(projectId?: string): Promise<RealtimeStats> {
  return fetchAPI<RealtimeStats>(withProject("/v1/analytics/realtime", projectId));
}
