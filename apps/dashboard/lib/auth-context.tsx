"use client";
/* eslint-disable react-hooks/set-state-in-effect -- intentional session validation on mount/refresh */

import { createContext, useCallback, useContext, useEffect, useMemo, useState, type ReactNode } from "react";
import {
  clearStoredSessionState,
  getMe,
  getStoredWorkspace,
  login as apiLogin,
  logout as apiLogout,
  register as apiRegister,
  setStoredWorkspace,
  type AuthUser,
  type WorkspaceMembership,
} from "@/lib/api";

interface AuthContextValue {
  user: AuthUser | null;
  workspaces: WorkspaceMembership[];
  selectedWorkspaceID: string | null;
  selectedMembership: WorkspaceMembership | null;
  loading: boolean;
  usingEnvKey: boolean;
  login: (email: string, password: string) => Promise<void>;
  register: (email: string, password: string, workspaceName?: string) => Promise<void>;
  logout: () => Promise<void>;
  selectWorkspace: (workspaceID: string) => void;
  refresh: () => void;
}

const AuthContext = createContext<AuthContextValue | null>(null);

const ENV_KEY_SET = (process.env.NEXT_PUBLIC_API_KEY || "") !== "";

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<AuthUser | null>(null);
  const [workspaces, setWorkspaces] = useState<WorkspaceMembership[]>([]);
  const [selectedWorkspaceID, setSelectedWorkspaceID] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [refreshToken, setRefreshToken] = useState(0);

  useEffect(() => {
    let cancelled = false;
    // Sessions moved from localStorage to an HttpOnly cookie: drop any
    // token a previous version left behind before validating the cookie.
    clearStoredSessionState();
    // The session cookie is HttpOnly, so this is the only way to learn
    // whether we are signed in: ask the API. A 401 here is quiet state,
    // not an error — the route guards decide where signed-out users go.
    if (ENV_KEY_SET) {
      setLoading(false);
      return;
    }
    getMe()
      .then(({ user: me, workspaces: memberships }) => {
        if (cancelled) return;
        setUser(me);
        setWorkspaces(memberships);
        const stored = getStoredWorkspace();
        const valid = memberships.some((m) => m.workspace_id === stored)
          ? stored
          : (memberships[0]?.workspace_id ?? null);
        setSelectedWorkspaceID(valid);
        setStoredWorkspace(valid ?? "");
      })
      .catch(() => {
        if (cancelled) return;
        clearStoredSessionState();
        setStoredWorkspace("");
        setUser(null);
        setWorkspaces([]);
        setSelectedWorkspaceID(null);
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [refreshToken]);

  const applySession = useCallback((nextUser: AuthUser, memberships: WorkspaceMembership[]) => {
    clearStoredSessionState();
    setUser(nextUser);
    setWorkspaces(memberships);
    const stored = getStoredWorkspace();
    const valid = memberships.some((m) => m.workspace_id === stored)
      ? stored
      : (memberships[0]?.workspace_id ?? null);
    setSelectedWorkspaceID(valid);
    setStoredWorkspace(valid ?? "");
  }, []);

  const login = useCallback(
    async (email: string, password: string) => {
      const response = await apiLogin(email, password);
      applySession(response.user, response.workspaces ?? []);
    },
    [applySession],
  );

  const register = useCallback(
    async (email: string, password: string, workspaceName?: string) => {
      // Registration no longer opens a session: the emailed verification
      // link is the front door. The caller redirects to /verify-email.
      await apiRegister(email, password, workspaceName);
    },
    [],
  );

  const logout = useCallback(async () => {
    try {
      await apiLogout();
    } finally {
      setStoredWorkspace("");
      setUser(null);
      setWorkspaces([]);
      setSelectedWorkspaceID(null);
      // Deliberate full reload: drops every cached report and provider
      // state along with the revoked session. AppShell would redirect
      // anyway, but this also re-runs the middleware cookie check.
      // eslint-disable-next-line @next/next/no-location-assign-relative-destination
      window.location.assign("/login");
    }
  }, []);

  const selectWorkspace = useCallback((workspaceID: string) => {
    setSelectedWorkspaceID(workspaceID);
    setStoredWorkspace(workspaceID);
  }, []);

  const refresh = useCallback(() => setRefreshToken((n) => n + 1), []);

  const value = useMemo<AuthContextValue>(() => {
    const selectedMembership = workspaces.find((m) => m.workspace_id === selectedWorkspaceID) ?? null;
    return {
      user,
      workspaces,
      selectedWorkspaceID,
      selectedMembership,
      loading,
      usingEnvKey: ENV_KEY_SET,
      login,
      register,
      logout,
      selectWorkspace,
      refresh,
    };
  }, [user, workspaces, selectedWorkspaceID, loading, login, register, logout, selectWorkspace, refresh]);

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export function useAuth(): AuthContextValue {
  const context = useContext(AuthContext);
  if (!context) throw new Error("useAuth must be used within AuthProvider");
  return context;
}
