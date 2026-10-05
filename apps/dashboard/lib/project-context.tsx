/* eslint-disable react-hooks/set-state-in-effect -- intentional project list fetch on mount/refresh */
"use client";

import { createContext, useCallback, useContext, useEffect, useMemo, useState, type ReactNode } from "react";
import { getProjects, rangeSpanDays, type Project, type ReportRange } from "@/lib/api";
import { useAuth } from "@/lib/auth-context";

export interface DateRangeOption {
  days: number;
  label: string;
  short: string;
}

export const DATE_RANGES: DateRangeOption[] = [
  { days: 1, label: "Last 24 hours", short: "24H" },
  { days: 7, label: "Last 7 days", short: "7D" },
  { days: 30, label: "Last 30 days", short: "30D" },
  { days: 90, label: "Last 90 days", short: "90D" },
];

export interface CustomRange {
  start: string;
  end: string;
}

const PROJECT_STORAGE_KEY = "devpulse.project";
const DAYS_STORAGE_KEY = "devpulse.days";
const CUSTOM_STORAGE_KEY = "devpulse.customRange";

const DATE_PATTERN = /^\d{4}-\d{2}-\d{2}$/;

// isValidCustomRange enforces the same rules as the API: strict YYYY-MM-DD
// dates, start <= end, and a span within one year.
export function isValidCustomRange(custom: CustomRange | null): custom is CustomRange {
  if (!custom) return false;
  if (!DATE_PATTERN.test(custom.start) || !DATE_PATTERN.test(custom.end)) return false;
  if (custom.start > custom.end) return false;
  const span = rangeSpanDays(custom.start, custom.end);
  return span >= 1 && span <= 365;
}

interface ProjectContextValue {
  projects: Project[];
  selectedTrackingId: string | null;
  selectedProject: Project | null;
  days: number;
  presetDays: number;
  range: ReportRange;
  customRange: CustomRange | null;
  loading: boolean;
  error: string | null;
  selectProject: (trackingId: string) => void;
  setDays: (days: number) => void;
  setCustomRange: (start: string, end: string) => void;
  refresh: () => void;
}

const ProjectContext = createContext<ProjectContextValue | null>(null);

function readURLParams(): { project: string | null; days: number | null; custom: CustomRange | null } {
  if (typeof window === "undefined") return { project: null, days: null, custom: null };
  const params = new URLSearchParams(window.location.search);
  const project = params.get("project");
  const rawDays = params.get("days");
  const days = rawDays ? Number(rawDays) : null;
  const start = params.get("start");
  const end = params.get("end");
  const candidate = start && end ? { start, end } : null;
  return {
    project,
    days: days && DATE_RANGES.some((r) => r.days === days) ? days : null,
    custom: isValidCustomRange(candidate) ? candidate : null,
  };
}

function writeURLParams(project: string | null, days: number, custom: CustomRange | null) {
  if (typeof window === "undefined") return;
  const params = new URLSearchParams(window.location.search);
  if (project) {
    params.set("project", project);
  } else {
    params.delete("project");
  }
  if (custom) {
    params.set("start", custom.start);
    params.set("end", custom.end);
    params.delete("days");
  } else {
    params.delete("start");
    params.delete("end");
    params.set("days", String(days));
  }
  const next = `${window.location.pathname}?${params.toString()}`;
  window.history.replaceState(null, "", next);
}

function readStoredCustom(): CustomRange | null {
  if (typeof window === "undefined") return null;
  const raw = window.localStorage.getItem(CUSTOM_STORAGE_KEY);
  if (!raw) return null;
  const [start, end] = raw.split(",");
  const candidate = start && end ? { start, end } : null;
  return isValidCustomRange(candidate) ? candidate : null;
}

function writeStoredCustom(custom: CustomRange | null) {
  if (typeof window === "undefined") return;
  if (custom) {
    window.localStorage.setItem(CUSTOM_STORAGE_KEY, `${custom.start},${custom.end}`);
  } else {
    window.localStorage.removeItem(CUSTOM_STORAGE_KEY);
  }
}

export function ProjectProvider({ children }: { children: ReactNode }) {
  const { selectedWorkspaceID, loading: authLoading } = useAuth();
  const [projects, setProjects] = useState<Project[]>([]);
  const [selectedTrackingId, setSelectedTrackingId] = useState<string | null>(null);
  const [days, setDaysState] = useState(30);
  const [customRange, setCustomRangeState] = useState<CustomRange | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [refreshToken, setRefreshToken] = useState(0);

  useEffect(() => {
    if (authLoading) return;
    let cancelled = false;
    setLoading(true);
    getProjects()
      .then((list) => {
        if (cancelled) return;
        setProjects(list);
        setError(null);
        const url = readURLParams();
        const storedDays = Number(
          typeof window !== "undefined" ? window.localStorage.getItem(DAYS_STORAGE_KEY) : null,
        );
        const initialDays = url.days ?? (DATE_RANGES.some((r) => r.days === storedDays) ? storedDays : 30);
        const initialCustom = url.custom ?? readStoredCustom();
        setDaysState(initialDays);
        setCustomRangeState(initialCustom);

        const storedProject =
          typeof window !== "undefined" ? window.localStorage.getItem(PROJECT_STORAGE_KEY) : null;
        const candidate = url.project ?? storedProject ?? list[0]?.tracking_id ?? null;
        const valid = list.some((p) => p.tracking_id === candidate) ? candidate : (list[0]?.tracking_id ?? null);
        setSelectedTrackingId(valid);
        writeURLParams(valid, initialDays, initialCustom);
      })
      .catch((e) => {
        if (!cancelled) setError(e.message);
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [refreshToken, selectedWorkspaceID, authLoading]);

  const selectProject = useCallback(
    (trackingId: string) => {
      setSelectedTrackingId(trackingId);
      if (typeof window !== "undefined") {
        window.localStorage.setItem(PROJECT_STORAGE_KEY, trackingId);
      }
      writeURLParams(trackingId, days, customRange);
    },
    [days, customRange],
  );

  const setDays = useCallback(
    (next: number) => {
      if (!DATE_RANGES.some((r) => r.days === next)) return;
      setDaysState(next);
      setCustomRangeState(null);
      if (typeof window !== "undefined") {
        window.localStorage.setItem(DAYS_STORAGE_KEY, String(next));
      }
      writeStoredCustom(null);
      writeURLParams(selectedTrackingId, next, null);
    },
    [selectedTrackingId],
  );

  const setCustomRange = useCallback(
    (start: string, end: string) => {
      const custom: CustomRange = { start, end };
      if (!isValidCustomRange(custom)) return;
      setCustomRangeState(custom);
      writeStoredCustom(custom);
      writeURLParams(selectedTrackingId, days, custom);
    },
    [selectedTrackingId, days],
  );

  const refresh = useCallback(() => setRefreshToken((n) => n + 1), []);

  const range = useMemo<ReportRange>(
    () => (customRange ? { start_date: customRange.start, end_date: customRange.end } : { days }),
    [customRange, days],
  );
  const effectiveDays = customRange ? rangeSpanDays(customRange.start, customRange.end) : days;

  const value = useMemo<ProjectContextValue>(() => {
    const selectedProject = projects.find((p) => p.tracking_id === selectedTrackingId) ?? null;
    return {
      projects,
      selectedTrackingId,
      selectedProject,
      days: effectiveDays,
      presetDays: days,
      range,
      customRange,
      loading,
      error,
      selectProject,
      setDays,
      setCustomRange,
      refresh,
    };
  }, [
    projects,
    selectedTrackingId,
    effectiveDays,
    days,
    range,
    customRange,
    loading,
    error,
    selectProject,
    setDays,
    setCustomRange,
    refresh,
  ]);

  return <ProjectContext.Provider value={value}>{children}</ProjectContext.Provider>;
}

export function useProject(): ProjectContextValue {
  const context = useContext(ProjectContext);
  if (!context) throw new Error("useProject must be used within ProjectProvider");
  return context;
}
