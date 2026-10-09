import { isPublicPath, safeNext } from "@/lib/paths";

// All dashboard calls go through the same-origin BFF proxy (app/api),
// which injects the HttpOnly session cookie as a Bearer token for the Go
// API. NEXT_PUBLIC_API_URL only matters server-side, inside that proxy.
const ENV_API_KEY = process.env.NEXT_PUBLIC_API_KEY || "";

const WORKSPACE_STORAGE_KEY = "devpulse.workspace";
// Sessions used to live here as a bearer token readable by any script.
// The key is cleared on boot so a stale credential cannot linger.
const LEGACY_TOKEN_STORAGE_KEY = "devpulse.token";

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

export function clearStoredSessionState() {
  if (typeof window === "undefined") return;
  window.localStorage.removeItem(LEGACY_TOKEN_STORAGE_KEY);
}

export class ApiError extends Error {
  readonly status: number;

  constructor(message: string, status: number) {
    super(message);
    this.name = "ApiError";
    this.status = status;
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

async function fetchAPI<T>(path: string, options: RequestInit = {}): Promise<T> {
  const headers: Record<string, string> = {
    "Content-Type": "application/json",
    ...(options.headers as Record<string, string> | undefined),
  };
  // Automation keys are the only browser-held credential; human sessions
  // travel automatically as an HttpOnly cookie on same-origin /api calls.
  if (!headers["Authorization"] && ENV_API_KEY) {
    headers["Authorization"] = `Bearer ${ENV_API_KEY}`;
  }
  if (!headers["X-Workspace-ID"]) {
    const workspaceID = getStoredWorkspace();
    if (workspaceID) {
      headers["X-Workspace-ID"] = workspaceID;
    }
  }
  const res = await fetch(`/api${path}`, {
    ...options,
    headers,
  });
  if (!res.ok) {
    const body = await res.json().catch(() => ({}));
    if (res.status === 401) redirectWhenSignedOut(path);
    throw new ApiError(body.error || `Request failed: ${res.status}`, res.status);
  }
  if (res.status === 204) {
    return undefined as T;
  }
  const text = await res.text();
  return (text ? JSON.parse(text) : undefined) as T;
}

// A 401 outside the auth endpoints means the session died mid-use
// (expired, revoked elsewhere). Auth endpoints own their errors instead:
// a wrong password on /login must render inline, not redirect.
function redirectWhenSignedOut(path: string) {
  if (typeof window === "undefined") return;
  if (ENV_API_KEY || path.startsWith("/v1/auth/")) return;
  const { pathname, search } = window.location;
  if (isPublicPath(pathname)) return;
  // Full reload on purpose: fetch code has no router, and re-bootstrapping
  // from scratch beats leaving stale report state behind a dead session.
  // eslint-disable-next-line @next/next/no-location-assign-relative-destination
  window.location.assign(`/login?next=${encodeURIComponent(safeNext(pathname + search))}`);
}

export function getProjects(): Promise<Project[]> {
  return fetchAPI<Project[]>("/v1/analytics/projects");
}

export interface CreateProjectInput {
  name: string;
  allowed_domains?: string[];
  timezone?: string;
  retention_days?: number;
}

export function createProject(input: CreateProjectInput): Promise<Project> {
  return fetchAPI<Project>("/v1/analytics/projects", {
    method: "POST",
    body: JSON.stringify(input),
  });
}

export interface UpdateProjectInput {
  name?: string;
  allowed_domains?: string[];
  timezone?: string;
  retention_days?: number;
  enabled?: boolean;
}

export function updateProject(id: string, input: UpdateProjectInput): Promise<Project> {
  return fetchAPI<Project>(`/v1/analytics/projects/${encodeURIComponent(id)}`, {
    method: "PATCH",
    body: JSON.stringify(input),
  });
}

export function deleteProject(id: string): Promise<void> {
  return fetchAPI<void>(`/v1/analytics/projects/${encodeURIComponent(id)}`, {
    method: "DELETE",
  });
}

function withProject(path: string, projectId?: string): string {
  if (!projectId) return path;
  const separator = path.includes("?") ? "&" : "?";
  return `${path}${separator}project_id=${encodeURIComponent(projectId)}`;
}

export interface ReportRange {
  days?: number;
  start_date?: string;
  end_date?: string;
}

export const MAX_RANGE_DAYS = 365;

export function rangeSpanDays(start: string, end: string): number {
  return Math.round((Date.parse(end) - Date.parse(start)) / 86_400_000) + 1;
}

function rangeQuery(range?: ReportRange | number): string {
  const normalized: ReportRange = typeof range === "number" || range == null ? { days: range ?? 30 } : range;
  if (normalized.start_date && normalized.end_date) {
    return `start_date=${encodeURIComponent(normalized.start_date)}&end_date=${encodeURIComponent(normalized.end_date)}`;
  }
  return `days=${normalized.days ?? 30}`;
}

export function getSummary(projectId?: string, range: ReportRange | number = 30): Promise<AnalyticsSummary> {
  return fetchAPI<AnalyticsSummary>(withProject(`/v1/analytics/summary?${rangeQuery(range)}`, projectId));
}

export function getTraffic(range: ReportRange | number = 30, projectId?: string): Promise<TrafficData[]> {
  return fetchAPI<TrafficData[]>(withProject(`/v1/analytics/traffic?${rangeQuery(range)}`, projectId));
}

export function getTopPages(limit = 20, projectId?: string, range: ReportRange | number = 30): Promise<TopPage[]> {
  return fetchAPI<TopPage[]>(withProject(`/v1/analytics/pages?limit=${limit}&${rangeQuery(range)}`, projectId));
}

export function getLandingPages(limit = 20, projectId?: string, range: ReportRange | number = 30): Promise<LandingPage[]> {
  return fetchAPI<LandingPage[]>(withProject(`/v1/analytics/landing-pages?limit=${limit}&${rangeQuery(range)}`, projectId));
}

export function getUTM(projectId?: string, range: ReportRange | number = 30): Promise<UTMReport> {
  return fetchAPI<UTMReport>(withProject(`/v1/analytics/utm?${rangeQuery(range)}`, projectId));
}

export function getSources(projectId?: string, range: ReportRange | number = 30): Promise<TrafficSource[]> {
  return fetchAPI<TrafficSource[]>(withProject(`/v1/analytics/sources?${rangeQuery(range)}`, projectId));
}

export function getCountries(projectId?: string, range: ReportRange | number = 30): Promise<CountryStats[]> {
  return fetchAPI<CountryStats[]>(withProject(`/v1/analytics/countries?${rangeQuery(range)}`, projectId));
}

export function getDevices(projectId?: string, range: ReportRange | number = 30): Promise<DevicesStats> {
  return fetchAPI<DevicesStats>(withProject(`/v1/analytics/devices?${rangeQuery(range)}`, projectId));
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
  /** Login returns every membership; register returns only the new workspace. */
  workspaces?: WorkspaceMembership[];
  /** Raw session token for direct API clients; stripped by the BFF proxy. */
  token?: string;
  expires_at?: string;
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

export function renameWorkspace(workspaceID: string, name: string): Promise<WorkspaceMembership> {
  return fetchAPI<WorkspaceMembership>(`/v1/workspaces/${workspaceID}`, {
    method: "PATCH",
    body: JSON.stringify({ name }),
  });
}

export function deleteWorkspace(workspaceID: string): Promise<void> {
  return fetchAPI<void>(`/v1/workspaces/${workspaceID}`, { method: "DELETE" });
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
