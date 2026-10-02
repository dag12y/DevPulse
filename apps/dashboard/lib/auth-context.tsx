"use client";
/* eslint-disable react-hooks/set-state-in-effect -- intentional session validation on mount/refresh */

import { createContext, useCallback, useContext, useEffect, useMemo, useState, type ReactNode } from "react";
import {
  getMe,
  getStoredToken,
  getStoredWorkspace,
  login as apiLogin,
  logout as apiLogout,
  register as apiRegister,
  setStoredToken,
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
    const token = getStoredToken();
    if (!token) {
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
        // Invalid or expired session: drop it so reports show sign-in state.
        setStoredToken("");
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

  const applySession = useCallback((nextUser: AuthUser, memberships: WorkspaceMembership[], token: string) => {
    setStoredToken(token);
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
      applySession(response.user, response.workspaces, response.token);
    },
    [applySession],
  );

  const register = useCallback(
    async (email: string, password: string, workspaceName?: string) => {
      const response = await apiRegister(email, password, workspaceName);
      applySession(response.user, response.workspaces ?? [response.workspace!], response.token);
    },
    [applySession],
  );

  const logout = useCallback(async () => {
    try {
      await apiLogout();
    } finally {
      setStoredToken("");
      setStoredWorkspace("");
      setUser(null);
      setWorkspaces([]);
      setSelectedWorkspaceID(null);
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
