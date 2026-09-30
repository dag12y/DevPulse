const API_URL = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";

export interface Project {
  id: string;
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

async function fetchAPI<T>(path: string, options?: RequestInit): Promise<T> {
  const res = await fetch(`${API_URL}${path}`, {
    ...options,
    headers: { "Content-Type": "application/json", ...options?.headers },
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

export function getSummary(): Promise<AnalyticsSummary> {
  return fetchAPI<AnalyticsSummary>("/v1/analytics/summary");
}

export function getTraffic(days = 30): Promise<TrafficData[]> {
  return fetchAPI<TrafficData[]>(`/v1/analytics/traffic?days=${days}`);
}

export function getTopPages(limit = 20): Promise<TopPage[]> {
  return fetchAPI<TopPage[]>(`/v1/analytics/pages?limit=${limit}`);
}
