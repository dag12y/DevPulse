/* eslint-disable react-hooks/set-state-in-effect -- intentional project list fetch on mount/refresh */
"use client";

import { createContext, useCallback, useContext, useEffect, useMemo, useState, type ReactNode } from "react";
import { getProjects, type Project } from "@/lib/api";

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

const PROJECT_STORAGE_KEY = "devpulse.project";
const DAYS_STORAGE_KEY = "devpulse.days";

interface ProjectContextValue {
  projects: Project[];
  selectedTrackingId: string | null;
  selectedProject: Project | null;
  days: number;
  loading: boolean;
  error: string | null;
  selectProject: (trackingId: string) => void;
  setDays: (days: number) => void;
  refresh: () => void;
}

const ProjectContext = createContext<ProjectContextValue | null>(null);

function readURLParams(): { project: string | null; days: number | null } {
  if (typeof window === "undefined") return { project: null, days: null };
  const params = new URLSearchParams(window.location.search);
  const project = params.get("project");
  const rawDays = params.get("days");
  const days = rawDays ? Number(rawDays) : null;
  return {
    project,
    days: days && DATE_RANGES.some((r) => r.days === days) ? days : null,
  };
}

function writeURLParams(project: string | null, days: number) {
  if (typeof window === "undefined") return;
  const params = new URLSearchParams(window.location.search);
  if (project) {
    params.set("project", project);
  } else {
    params.delete("project");
  }
  params.set("days", String(days));
  const next = `${window.location.pathname}?${params.toString()}`;
  window.history.replaceState(null, "", next);
}

export function ProjectProvider({ children }: { children: ReactNode }) {
  const [projects, setProjects] = useState<Project[]>([]);
  const [selectedTrackingId, setSelectedTrackingId] = useState<string | null>(null);
  const [days, setDaysState] = useState(30);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [refreshToken, setRefreshToken] = useState(0);

  useEffect(() => {
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
        setDaysState(initialDays);

        const storedProject =
          typeof window !== "undefined" ? window.localStorage.getItem(PROJECT_STORAGE_KEY) : null;
        const candidate = url.project ?? storedProject ?? list[0]?.tracking_id ?? null;
        const valid = list.some((p) => p.tracking_id === candidate) ? candidate : (list[0]?.tracking_id ?? null);
        setSelectedTrackingId(valid);
        writeURLParams(valid, initialDays);
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
  }, [refreshToken]);

  const selectProject = useCallback(
    (trackingId: string) => {
      setSelectedTrackingId(trackingId);
      if (typeof window !== "undefined") {
        window.localStorage.setItem(PROJECT_STORAGE_KEY, trackingId);
      }
      writeURLParams(trackingId, days);
    },
    [days],
  );

  const setDays = useCallback(
    (next: number) => {
      if (!DATE_RANGES.some((r) => r.days === next)) return;
      setDaysState(next);
      if (typeof window !== "undefined") {
        window.localStorage.setItem(DAYS_STORAGE_KEY, String(next));
      }
      writeURLParams(selectedTrackingId, next);
    },
    [selectedTrackingId],
  );

  const refresh = useCallback(() => setRefreshToken((n) => n + 1), []);

  const value = useMemo<ProjectContextValue>(() => {
    const selectedProject = projects.find((p) => p.tracking_id === selectedTrackingId) ?? null;
    return {
      projects,
      selectedTrackingId,
      selectedProject,
      days,
      loading,
      error,
      selectProject,
      setDays,
      refresh,
    };
  }, [projects, selectedTrackingId, days, loading, error, selectProject, setDays, refresh]);

  return <ProjectContext.Provider value={value}>{children}</ProjectContext.Provider>;
}

export function useProject(): ProjectContextValue {
  const context = useContext(ProjectContext);
  if (!context) throw new Error("useProject must be used within ProjectProvider");
  return context;
}
