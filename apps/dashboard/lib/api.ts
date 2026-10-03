const API_URL = process.env.NEXT_PUBLIC_API_URL || "http://localhost:5000";
const API_KEY = process.env.NEXT_PUBLIC_API_KEY || "";

const TOKEN_STORAGE_KEY = "devpulse.token";
const WORKSPACE_STORAGE_KEY = "devpulse.workspace";

export function getStoredToken(): string {
  if (typeof window === "undefined") return "";
  return window.localStorage.getItem(TOKEN_STORAGE_KEY) || "";
}

export function setStoredToken(token: string) {
  if (typeof window === "undefined") return;
  if (token) {
    window.localStorage.setItem(TOKEN_STORAGE_KEY, token);
  } else {
    window.localStorage.removeItem(TOKEN_STORAGE_KEY);
  }
}

export function getStoredWorkspace(): string {
  if (typeof window === "undefined") return "";
  return window.localStorage.getItem(WORKSPACE_STORAGE_KEY) || "";
}

export function setStoredWorkspace(workspaceID: string) {
  if (typeof window === "undefined") return;
  if (workspaceID) {
    window.localStorage.setItem(WORKSPACE_STORAGE_KEY, workspaceID);
  } else {
    window.localStorage.removeItem(WORKSPACE_STORAGE_KEY);
  }
}

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
  days: number;
  prev_page_views: number;
  prev_visitors: number;
  prev_sessions: number;
  page_views_change: number | null;
  visitors_change: number | null;
  sessions_change: number | null;
  prev_bounce_rate: number;
  bounce_rate_change: number;
  new_visitors: number;
  returning_visitors: number;
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

export interface LandingPage {
  path: string;
  sessions: number;
  visitors: number;
  share: number;
}

export interface UTMBreakdown {
  name: string;
  page_views: number;
  visitors: number;
  percentage: number;
}

export interface UTMReport {
  sources: UTMBreakdown[];
  mediums: UTMBreakdown[];
  campaigns: UTMBreakdown[];
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
  screens: DeviceBreakdown[];
  viewports: DeviceBreakdown[];
}

async function fetchAPI<T>(path: string, options?: RequestInit): Promise<T> {
  const headers: Record<string, string> = { "Content-Type": "application/json", ...options?.headers as Record<string, string> };
  if (!headers["Authorization"]) {
    // Automation keys (env) take precedence; otherwise use the login session.
    const token = API_KEY || getStoredToken();
    if (token) {
      headers["Authorization"] = `Bearer ${token}`;
    }
  }
  if (!headers["X-Workspace-ID"]) {
    const workspaceID = getStoredWorkspace();
    if (workspaceID) {
      headers["X-Workspace-ID"] = workspaceID;
    }
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

export function getSummary(projectId?: string, days = 30): Promise<AnalyticsSummary> {
  return fetchAPI<AnalyticsSummary>(withProject(`/v1/analytics/summary?days=${days}`, projectId));
}

export function getTraffic(days = 30, projectId?: string): Promise<TrafficData[]> {
  return fetchAPI<TrafficData[]>(withProject(`/v1/analytics/traffic?days=${days}`, projectId));
}

export function getTopPages(limit = 20, projectId?: string, days = 30): Promise<TopPage[]> {
  return fetchAPI<TopPage[]>(withProject(`/v1/analytics/pages?limit=${limit}&days=${days}`, projectId));
}

export function getLandingPages(limit = 20, projectId?: string, days = 30): Promise<LandingPage[]> {
  return fetchAPI<LandingPage[]>(withProject(`/v1/analytics/landing-pages?limit=${limit}&days=${days}`, projectId));
}

export function getUTM(projectId?: string, days = 30): Promise<UTMReport> {
  return fetchAPI<UTMReport>(withProject(`/v1/analytics/utm?days=${days}`, projectId));
}

export function getSources(projectId?: string, days = 30): Promise<TrafficSource[]> {
  return fetchAPI<TrafficSource[]>(withProject(`/v1/analytics/sources?days=${days}`, projectId));
}

export function getCountries(projectId?: string, days = 30): Promise<CountryStats[]> {
  return fetchAPI<CountryStats[]>(withProject(`/v1/analytics/countries?days=${days}`, projectId));
}

export function getDevices(projectId?: string, days = 30): Promise<DevicesStats> {
  return fetchAPI<DevicesStats>(withProject(`/v1/analytics/devices?days=${days}`, projectId));
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

export interface AuthUser {
  id: string;
  email: string;
  created_at: string;
}

export interface WorkspaceMembership {
  workspace_id: string;
  workspace_name: string;
  role: string;
}

export interface WorkspaceMember {
  user_id: string;
  email: string;
  role: string;
}

export interface AuthResponse {
  user: AuthUser;
  workspaces: WorkspaceMembership[];
  token: string;
  expires_at: string;
  workspace?: WorkspaceMembership;
}

export function register(email: string, password: string, workspaceName?: string): Promise<AuthResponse> {
  return fetchAPI<AuthResponse>("/v1/auth/register", {
    method: "POST",
    body: JSON.stringify({ email, password, workspace_name: workspaceName || undefined }),
  });
}

export function login(email: string, password: string): Promise<AuthResponse> {
  return fetchAPI<AuthResponse>("/v1/auth/login", {
    method: "POST",
    body: JSON.stringify({ email, password }),
  });
}

export function logout(): Promise<void> {
  return fetchAPI<void>("/v1/auth/logout", { method: "POST" }).catch(() => undefined);
}

export function getMe(): Promise<{ user: AuthUser; workspaces: WorkspaceMembership[] }> {
  return fetchAPI<{ user: AuthUser; workspaces: WorkspaceMembership[] }>("/v1/auth/me");
}

export function listMyWorkspaces(): Promise<WorkspaceMembership[]> {
  return fetchAPI<WorkspaceMembership[]>("/v1/workspaces");
}

export function createWorkspace(name: string): Promise<WorkspaceMembership> {
  return fetchAPI<WorkspaceMembership>("/v1/workspaces", {
    method: "POST",
    body: JSON.stringify({ name }),
  });
}

export function listMembers(workspaceID: string): Promise<WorkspaceMember[]> {
  return fetchAPI<WorkspaceMember[]>(`/v1/workspaces/${workspaceID}/members`);
}

export function addMember(workspaceID: string, email: string, role: string): Promise<WorkspaceMember> {
  return fetchAPI<WorkspaceMember>(`/v1/workspaces/${workspaceID}/members`, {
    method: "POST",
    body: JSON.stringify({ email, role }),
  });
}

export function updateMemberRole(workspaceID: string, userID: string, role: string): Promise<WorkspaceMember> {
  return fetchAPI<WorkspaceMember>(`/v1/workspaces/${workspaceID}/members/${userID}`, {
    method: "PATCH",
    body: JSON.stringify({ role }),
  });
}

export function removeMember(workspaceID: string, userID: string): Promise<void> {
  return fetchAPI<void>(`/v1/workspaces/${workspaceID}/members/${userID}`, { method: "DELETE" });
}
