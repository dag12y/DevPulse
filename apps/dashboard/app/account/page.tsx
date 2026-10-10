"use client";
/* eslint-disable react-hooks/set-state-in-effect -- intentional refetch when workspace changes */

import { useCallback, useEffect, useState } from "react";
import Link from "next/link";
import { useAuth } from "@/lib/auth-context";
import {
  addMember,
  createWorkspace,
  deleteWorkspace,
  disableTwoFactor,
  enableTwoFactor,
  listMembers,
  listSessions,
  removeMember,
  renameWorkspace,
  revokeOtherSessions,
  revokeSession,
  setupTwoFactor,
  updateMemberRole,
  type SessionInfo,
  type TwoFactorSetup,
  type WorkspaceMember,
} from "@/lib/api";
import Badge from "@/components/ui/Badge";
import Card from "@/components/ui/Card";
import PageHeader from "@/components/ui/PageHeader";
import { friendlyAuthError } from "@/components/ui/auth-errors";

const inputClass =
  "rounded-lg border border-zinc-300 bg-white px-3 py-2 text-sm text-zinc-900 shadow-xs outline-none transition placeholder:text-zinc-400 focus:border-indigo-500 focus:ring-4 focus:ring-indigo-500/15 dark:border-zinc-700 dark:bg-zinc-900 dark:text-zinc-100 dark:placeholder:text-zinc-500";

const btnSecondary =
  "rounded-lg border border-zinc-300 px-3 py-2 text-sm font-medium hover:bg-zinc-50 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-indigo-600 disabled:opacity-50 dark:border-zinc-700 dark:hover:bg-zinc-800";

const btnPrimary =
  "rounded-lg bg-indigo-600 px-4 py-2 text-sm font-semibold text-white shadow-sm shadow-indigo-600/25 hover:bg-indigo-500 disabled:opacity-50 dark:bg-indigo-500 dark:hover:bg-indigo-400";

/**
 * Two-factor enrollment and removal. Enrollment is two steps (generate a
 * secret, prove a code from it) so abandoning setup mid-way never locks
 * anyone out, and removal demands a live code so a stolen session alone
 * cannot strip the factor.
 */
function TwoFactorCard({ enabled }: { enabled: boolean }) {
  const { refresh } = useAuth();
  const [setup, setSetup] = useState<TwoFactorSetup | null>(null);
  const [code, setCode] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [confirmDisable, setConfirmDisable] = useState(false);

  const startSetup = async () => {
    if (busy) return;
    setBusy(true);
    setError(null);
    setCode("");
    try {
      setSetup(await setupTwoFactor());
    } catch (err) {
      setError(friendlyAuthError(err instanceof Error ? err.message : "", "Couldn't start setup. Try again."));
    } finally {
      setBusy(false);
    }
  };

  const enable = async (e: React.FormEvent) => {
    e.preventDefault();
    if (busy) return;
    setBusy(true);
    setError(null);
    try {
      await enableTwoFactor(code);
      setSetup(null);
      setCode("");
      await refresh();
    } catch (err) {
      setError(friendlyAuthError(err instanceof Error ? err.message : "", "Couldn't enable two-factor. Try again."));
    } finally {
      setBusy(false);
    }
  };

  const disable = async (e: React.FormEvent) => {
    e.preventDefault();
    if (busy) return;
    setBusy(true);
    setError(null);
    try {
      await disableTwoFactor(code);
      setConfirmDisable(false);
      setCode("");
      await refresh();
    } catch (err) {
      setError(friendlyAuthError(err instanceof Error ? err.message : "", "Couldn't disable two-factor. Try again."));
    } finally {
      setBusy(false);
    }
  };

  const cancel = () => {
    setSetup(null);
    setConfirmDisable(false);
    setCode("");
    setError(null);
  };

  return (
    <Card as="section" className="max-w-3xl space-y-3">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <h3 className="font-semibold text-zinc-900 dark:text-white">Two-factor authentication</h3>
        <Badge tone={enabled ? "success" : "neutral"}>{enabled ? "enabled" : "off"}</Badge>
      </div>
      {error && (
        <p role="alert" className="rounded-lg border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700 dark:border-red-800/60 dark:bg-red-950/40 dark:text-red-300">
          {error}
        </p>
      )}

      {enabled ? (
        confirmDisable ? (
          <form onSubmit={disable} className="space-y-3">
            <p className="text-sm text-zinc-600 dark:text-zinc-400">
              Enter your current authentication code to turn two-factor off. Until then, signing in still requires it.
            </p>
            <div className="flex flex-wrap items-end gap-2">
              <input
                inputMode="numeric"
                maxLength={6}
                autoComplete="one-time-code"
                placeholder="123456"
                aria-label="Authentication code"
                value={code}
                onChange={(e) => setCode(e.target.value.replace(/\D/g, ""))}
                className={`${inputClass} w-40 font-mono`}
              />
              <button type="submit" disabled={busy || !/^\d{6}$/.test(code)} className="rounded-lg bg-red-600 px-4 py-2 text-sm font-semibold text-white hover:bg-red-500 disabled:opacity-50">
                {busy ? "Turning off..." : "Turn off two-factor"}
              </button>
              <button type="button" onClick={cancel} className={btnSecondary}>
                Cancel
              </button>
            </div>
          </form>
        ) : (
          <div className="space-y-2">
            <p className="text-sm text-zinc-600 dark:text-zinc-400">
              Sign-in asks for a 6-digit code from your authenticator app in addition to your password.
            </p>
            <button onClick={() => setConfirmDisable(true)} className={btnSecondary}>
              Turn off
            </button>
          </div>
        )
      ) : setup ? (
        <div className="space-y-3">
          <ol className="list-decimal space-y-1 pl-5 text-sm text-zinc-600 dark:text-zinc-400">
            <li>Scan the QR code with Google Authenticator, 1Password, Authy, or any TOTP app.</li>
            <li>Enter the 6-digit code it shows to confirm.</li>
          </ol>
          <div className="flex flex-wrap items-start gap-4">
            {/* eslint-disable-next-line @next/next/no-img-element -- QR is generated server-side as a PNG; no static asset to serve. */}
            <img src={`data:image/png;base64,${setup.qr_png_base64}`} alt="Two-factor enrollment QR code" width={168} height={168} className="rounded-lg border border-zinc-200 bg-white p-1 dark:border-zinc-700" />
            <div className="min-w-0 space-y-1">
              <p className="text-xs font-semibold uppercase tracking-wide text-zinc-500">Or enter this key</p>
              <p className="break-all rounded-lg border border-zinc-200 bg-zinc-50 px-2 py-1.5 font-mono text-xs text-zinc-700 dark:border-zinc-700 dark:bg-zinc-900 dark:text-zinc-300">
                {setup.secret}
              </p>
            </div>
          </div>
          <form onSubmit={enable} className="flex flex-wrap items-end gap-2">
            <input
              inputMode="numeric"
              maxLength={6}
              autoComplete="one-time-code"
              placeholder="123456"
              aria-label="Authentication code"
              value={code}
              onChange={(e) => setCode(e.target.value.replace(/\D/g, ""))}
              className={`${inputClass} w-40 font-mono`}
            />
            <button type="submit" disabled={busy || !/^\d{6}$/.test(code)} className={btnPrimary}>
              {busy ? "Confirming..." : "Confirm and enable"}
            </button>
            <button type="button" onClick={cancel} className={btnSecondary}>
              Cancel
            </button>
          </form>
        </div>
      ) : (
        <div className="space-y-2">
          <p className="text-sm text-zinc-600 dark:text-zinc-400">
            Add a second step at sign-in: a rotating 6-digit code from your phone. Even if someone learns your password, they can&apos;t get in without it.
          </p>
          <button onClick={startSetup} disabled={busy} className={btnPrimary}>
            {busy ? "Preparing..." : "Set up two-factor"}
          </button>
        </div>
      )}
    </Card>
  );
}

/** Live sessions for this account, with per-row and "everything else" revocation. */
function SessionsCard() {
  const [sessions, setSessions] = useState<SessionInfo[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  const load = useCallback(async () => {
    setError(null);
    try {
      setSessions(await listSessions());
    } catch (err) {
      setError(friendlyAuthError(err instanceof Error ? err.message : "", "Couldn't load sessions."));
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    load();
  }, [load]);

  const revoke = async (id: string) => {
    if (busy) return;
    setBusy(true);
    setError(null);
    try {
      await revokeSession(id);
      await load();
    } catch (err) {
      setError(friendlyAuthError(err instanceof Error ? err.message : "", "Couldn't sign that session out."));
    } finally {
      setBusy(false);
    }
  };

  const revokeOthers = async () => {
    if (busy) return;
    setBusy(true);
    setError(null);
    try {
      await revokeOtherSessions();
      await load();
    } catch (err) {
      setError(friendlyAuthError(err instanceof Error ? err.message : "", "Couldn't sign other sessions out."));
    } finally {
      setBusy(false);
    }
  };

  const others = sessions.filter((session) => !session.current);

  return (
    <Card as="section" className="max-w-3xl space-y-3">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <h3 className="font-semibold text-zinc-900 dark:text-white">Active sessions</h3>
        {others.length > 0 && (
          <button onClick={revokeOthers} disabled={busy} className={btnSecondary}>
            Sign out other sessions
          </button>
        )}
      </div>
      {error && (
        <p role="alert" className="rounded-lg border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700 dark:border-red-800/60 dark:bg-red-950/40 dark:text-red-300">
          {error} <button className="underline" onClick={load}>Retry</button>
        </p>
      )}
      {loading ? (
        <div className="space-y-2" role="status" aria-label="Loading sessions">
          <div className="h-10 animate-pulse rounded-lg bg-zinc-200 dark:bg-zinc-800" />
          <div className="h-10 animate-pulse rounded-lg bg-zinc-200 dark:bg-zinc-800" />
        </div>
      ) : sessions.length === 0 ? (
        <p className="text-sm text-zinc-500">No active sessions found.</p>
      ) : (
        <ul className="space-y-2">
          {sessions.map((session) => (
            <li key={session.id} className="flex flex-wrap items-center justify-between gap-2 rounded-xl border border-zinc-200 p-3 dark:border-zinc-800">
              <div className="min-w-0 space-y-0.5">
                <div className="flex flex-wrap items-center gap-2">
                  <span className="truncate text-sm font-medium text-zinc-900 dark:text-zinc-100">
                    {session.user_agent || "Unknown device"}
                  </span>
                  {session.current && <Badge tone="indigo">this device</Badge>}
                </div>
                <p className="text-xs text-zinc-500">
                  {session.ip && <>IP {session.ip} · </>}
                  Signed in {new Date(session.created_at).toLocaleString()}
                  {session.last_seen_at && <> · last active {new Date(session.last_seen_at).toLocaleString()}</>}
                </p>
              </div>
              {!session.current && (
                <button
                  onClick={() => revoke(session.id)}
                  disabled={busy}
                  className="rounded-lg border border-zinc-300 px-3 py-1.5 text-xs font-medium text-red-700 hover:bg-red-50 disabled:opacity-50 dark:border-zinc-700 dark:text-red-300 dark:hover:bg-red-950/40"
                >
                  Sign out
                </button>
              )}
            </li>
          ))}
        </ul>
      )}
      <p className="text-xs text-zinc-500">
        Signing out other sessions ends them everywhere except this device.
      </p>
    </Card>
  );
}

export default function AccountPage() {
  const { user, workspaces, selectedWorkspaceID, selectWorkspace, selectedMembership, logout, refresh, loading } =
    useAuth();
  const [members, setMembers] = useState<WorkspaceMember[]>([]);
  const [membersError, setMembersError] = useState<string | null>(null);
  const [inviteEmail, setInviteEmail] = useState("");
  const [inviteRole, setInviteRole] = useState("viewer");
  const [actionError, setActionError] = useState<string | null>(null);
  const [inviting, setInviting] = useState(false);
  const [newWorkspace, setNewWorkspace] = useState("");
  const [creating, setCreating] = useState(false);
  const [editingWorkspaceID, setEditingWorkspaceID] = useState<string | null>(null);
  const [editingName, setEditingName] = useState("");
  const [confirmDeleteID, setConfirmDeleteID] = useState<string | null>(null);

  useEffect(() => {
    if (!selectedWorkspaceID) {
      setMembers([]);
      return;
    }
    let cancelled = false;
    setMembersError(null);
    listMembers(selectedWorkspaceID)
      .then((list) => {
        if (!cancelled) setMembers(list);
      })
      .catch((e) => {
        if (!cancelled) setMembersError(e.message);
      });
    return () => {
      cancelled = true;
    };
  }, [selectedWorkspaceID]);

  if (loading) {
    return (
      <div className="p-6" role="status" aria-label="Loading account">
        <div className="max-w-3xl space-y-3">
          <div className="h-7 w-40 animate-pulse rounded-lg bg-zinc-200 dark:bg-zinc-800" />
          <div className="h-4 w-64 animate-pulse rounded bg-zinc-200 dark:bg-zinc-800" />
          <div className="h-40 animate-pulse rounded-2xl bg-zinc-200 dark:bg-zinc-800" />
        </div>
      </div>
    );
  }

  if (!user) {
    return (
      <div className="p-6 space-y-3">
        <h2 className="text-2xl font-bold tracking-tight text-zinc-900 dark:text-white">Account</h2>
        <p className="text-sm text-zinc-500">
          <Link className="font-medium text-indigo-600 underline-offset-2 hover:underline dark:text-indigo-400" href="/login">Sign in</Link> to manage workspaces and members.
        </p>
      </div>
    );
  }

  const canManage = selectedMembership && selectedMembership.role !== "viewer";

  const reloadMembers = () => {
    if (!selectedWorkspaceID) return;
    listMembers(selectedWorkspaceID).then(setMembers).catch((e) => setMembersError(e.message));
  };

  const invite = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!selectedWorkspaceID || inviting) return;
    setInviting(true);
    setActionError(null);
    try {
      await addMember(selectedWorkspaceID, inviteEmail.trim(), inviteRole);
      setInviteEmail("");
      reloadMembers();
    } catch (err) {
      setActionError(err instanceof Error ? err.message : "Invite failed");
    } finally {
      setInviting(false);
    }
  };

  const changeRole = async (userID: string, role: string) => {
    if (!selectedWorkspaceID) return;
    setActionError(null);
    try {
      await updateMemberRole(selectedWorkspaceID, userID, role);
      reloadMembers();
    } catch (err) {
      setActionError(err instanceof Error ? err.message : "Role change failed");
    }
  };

  const remove = async (userID: string) => {
    if (!selectedWorkspaceID) return;
    setActionError(null);
    try {
      await removeMember(selectedWorkspaceID, userID);
      reloadMembers();
    } catch (err) {
      setActionError(err instanceof Error ? err.message : "Removal failed");
    }
  };

  const create = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!newWorkspace.trim() || creating) return;
    setCreating(true);
    setActionError(null);
    try {
      await createWorkspace(newWorkspace.trim());
      setNewWorkspace("");
      await refresh();
    } catch (err) {
      setActionError(err instanceof Error ? err.message : "Create failed");
    } finally {
      setCreating(false);
    }
  };

  const startEditing = (workspaceID: string, name: string) => {
    setEditingWorkspaceID(workspaceID);
    setEditingName(name);
  };

  const saveRename = async (workspaceID: string) => {
    if (!editingName.trim()) return;
    setActionError(null);
    try {
      await renameWorkspace(workspaceID, editingName.trim());
      setEditingWorkspaceID(null);
      await refresh();
    } catch (err) {
      setActionError(err instanceof Error ? err.message : "Rename failed");
    }
  };

  const confirmDelete = async (workspaceID: string) => {
    setActionError(null);
    try {
      await deleteWorkspace(workspaceID);
      setConfirmDeleteID(null);
      await refresh();
    } catch (err) {
      setActionError(err instanceof Error ? err.message : "Delete failed");
    }
  };

  return (
    <div className="p-6 space-y-6">
      <PageHeader
        title="Account"
        description={user.email}
        actions={
          <button onClick={() => logout()} className={btnSecondary}>
            Sign out
          </button>
        }
      />

      {actionError && (
        <p role="alert" className="max-w-3xl rounded-xl border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700 dark:border-red-800/60 dark:bg-red-950/40 dark:text-red-300">
          {actionError}
        </p>
      )}

      <Card as="section" className="max-w-3xl space-y-3">
        <h3 className="font-semibold text-zinc-900 dark:text-white">Workspaces</h3>
        {workspaces.length === 0 && <p className="text-sm text-zinc-500">No workspaces yet. Create one below.</p>}
        <ul className="space-y-2">
          {workspaces.map((membership) => {
            const isSelected = membership.workspace_id === selectedWorkspaceID;
            const isEditing = editingWorkspaceID === membership.workspace_id;
            const isConfirmingDelete = confirmDeleteID === membership.workspace_id;
            return (
              <li
                key={membership.workspace_id}
                className={`rounded-xl border p-3 transition ${isSelected ? "border-indigo-300 bg-indigo-50/50 dark:border-indigo-700 dark:bg-indigo-500/5" : "border-zinc-200 dark:border-zinc-800"}`}
              >
                <div className="flex flex-wrap items-center justify-between gap-2">
                  <div className="min-w-0">
                    {isEditing ? (
                      <span className="flex gap-2">
                        <input
                          value={editingName}
                          onChange={(e) => setEditingName(e.target.value)}
                          maxLength={255}
                          aria-label="Workspace name"
                          autoFocus
                          className={inputClass}
                        />
                        <button onClick={() => saveRename(membership.workspace_id)} disabled={!editingName.trim()} className={btnSecondary}>
                          Save
                        </button>
                        <button onClick={() => setEditingWorkspaceID(null)} className={btnSecondary}>
                          Cancel
                        </button>
                      </span>
                    ) : (
                      <span className="flex items-center gap-2">
                        <span className="truncate font-medium text-zinc-900 dark:text-zinc-100">
                          {membership.workspace_name}
                        </span>
                        <Badge tone={membership.role === "owner" ? "indigo" : "neutral"}>{membership.role}</Badge>
                        {isSelected && <Badge tone="success">current</Badge>}
                      </span>
                    )}
                  </div>
                  {!isEditing && !isConfirmingDelete && (
                    <span className="flex gap-1.5 text-xs">
                      {!isSelected && (
                        <button onClick={() => selectWorkspace(membership.workspace_id)} className={btnSecondary}>
                          Switch
                        </button>
                      )}
                      {canManage && (
                        <button onClick={() => startEditing(membership.workspace_id, membership.workspace_name)} className={btnSecondary}>
                          Rename
                        </button>
                      )}
                      {membership.role === "owner" && (
                        <button onClick={() => setConfirmDeleteID(membership.workspace_id)} className="rounded-lg border border-red-200 px-3 py-2 text-sm font-medium text-red-700 hover:bg-red-50 dark:border-red-800/60 dark:text-red-300 dark:hover:bg-red-950/40">
                          Delete
                        </button>
                      )}
                    </span>
                  )}
                </div>
                {isConfirmingDelete && (
                  <div className="mt-3 rounded-xl border border-red-200 bg-red-50 p-3 text-sm dark:border-red-800/60 dark:bg-red-950/30">
                    <p className="text-red-800 dark:text-red-200">
                      Delete <strong>{membership.workspace_name}</strong>? Projects, members, and analytics go with it. This cannot be undone.
                    </p>
                    <div className="mt-2 flex gap-2">
                      <button onClick={() => confirmDelete(membership.workspace_id)} className="rounded-lg bg-red-600 px-3 py-1.5 text-sm font-semibold text-white hover:bg-red-500">
                        Yes, delete
                      </button>
                      <button onClick={() => setConfirmDeleteID(null)} className={btnSecondary}>
                        Keep
                      </button>
                    </div>
                  </div>
                )}
              </li>
            );
          })}
        </ul>
        <form onSubmit={create} className="flex flex-wrap gap-2">
          <input
            value={newWorkspace}
            onChange={(e) => setNewWorkspace(e.target.value)}
            placeholder="New workspace name"
            maxLength={255}
            aria-label="New workspace name"
            className={`${inputClass} flex-1`}
          />
          <button type="submit" disabled={!newWorkspace.trim() || creating} className={btnPrimary}>
            {creating ? "Creating..." : "Create workspace"}
          </button>
        </form>
      </Card>

      <Card as="section" className="max-w-3xl space-y-3">
        <h3 className="font-semibold text-zinc-900 dark:text-white">Members</h3>
        {membersError ? (
          <p role="alert" className="text-sm text-red-600 dark:text-red-400">
            {membersError} <button className="underline" onClick={reloadMembers}>Retry</button>
          </p>
        ) : members.length === 0 ? (
          <p className="text-sm text-zinc-500">No members listed yet.</p>
        ) : (
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-zinc-200 text-left text-xs font-semibold uppercase tracking-wide text-zinc-500 dark:border-zinc-800 dark:text-zinc-400">
                <th className="pb-2">Email</th>
                <th className="pb-2">Role</th>
                <th className="pb-2 text-right">Actions</th>
              </tr>
            </thead>
            <tbody>
              {members.map((member) => (
                <tr key={member.user_id} className="border-b border-zinc-100 last:border-0 dark:border-zinc-800/50">
                  <td className="py-2.5 text-zinc-800 dark:text-zinc-200">{member.email}</td>
                  <td className="py-2.5">
                    {canManage ? (
                      <select
                        aria-label={`Role for ${member.email}`}
                        value={member.role}
                        onChange={(e) => changeRole(member.user_id, e.target.value)}
                        className="rounded-lg border border-zinc-300 bg-white px-2 py-1.5 text-sm dark:border-zinc-700 dark:bg-zinc-900"
                      >
                        <option value="owner">owner</option>
                        <option value="admin">admin</option>
                        <option value="viewer">viewer</option>
                      </select>
                    ) : (
                      <Badge tone="neutral">{member.role}</Badge>
                    )}
                  </td>
                  <td className="py-2.5 text-right">
                    {(canManage || member.user_id === user.id) && (
                      <button onClick={() => remove(member.user_id)} className="rounded-lg border px-2.5 py-1.5 text-xs font-medium hover:bg-zinc-50 dark:hover:bg-zinc-800">
                        {member.user_id === user.id ? "Leave" : "Remove"}
                      </button>
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
        {canManage && selectedWorkspaceID && (
          <form onSubmit={invite} className="mt-2 flex flex-wrap gap-2">
            <input
              type="email"
              required
              value={inviteEmail}
              onChange={(e) => setInviteEmail(e.target.value)}
              placeholder="teammate@example.com"
              aria-label="Invite email"
              disabled={inviting}
              className={`${inputClass} flex-1`}
            />
            <select
              aria-label="Invite role"
              value={inviteRole}
              onChange={(e) => setInviteRole(e.target.value)}
              disabled={inviting}
              className={inputClass}
            >
              <option value="viewer">viewer</option>
              <option value="admin">admin</option>
              <option value="owner">owner</option>
            </select>
            <button type="submit" disabled={inviting} className={btnPrimary}>
              {inviting ? "Inviting..." : "Invite"}
            </button>
          </form>
        )}
        <p className="text-xs text-zinc-500">
          Invites only work for registered emails. The last owner cannot be demoted or removed.
        </p>
      </Card>

      <TwoFactorCard enabled={Boolean(user.totp_enabled)} />
      <SessionsCard />
    </div>
  );
}
