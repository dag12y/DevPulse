"use client";
/* eslint-disable react-hooks/set-state-in-effect -- intentional refetch when workspace changes */

import { useEffect, useState } from "react";
import Link from "next/link";
import { useAuth } from "@/lib/auth-context";
import {
  addMember,
  createWorkspace,
  deleteWorkspace,
  listMembers,
  removeMember,
  renameWorkspace,
  updateMemberRole,
  type WorkspaceMember,
} from "@/lib/api";
import Badge from "@/components/ui/Badge";
import Card from "@/components/ui/Card";
import PageHeader from "@/components/ui/PageHeader";

const inputClass =
  "rounded-lg border border-zinc-300 bg-white px-3 py-2 text-sm text-zinc-900 shadow-xs outline-none transition placeholder:text-zinc-400 focus:border-indigo-500 focus:ring-4 focus:ring-indigo-500/15 dark:border-zinc-700 dark:bg-zinc-900 dark:text-zinc-100 dark:placeholder:text-zinc-500";

const btnSecondary =
  "rounded-lg border border-zinc-300 px-3 py-2 text-sm font-medium hover:bg-zinc-50 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-indigo-600 disabled:opacity-50 dark:border-zinc-700 dark:hover:bg-zinc-800";

const btnPrimary =
  "rounded-lg bg-indigo-600 px-4 py-2 text-sm font-semibold text-white shadow-sm shadow-indigo-600/25 hover:bg-indigo-500 disabled:opacity-50 dark:bg-indigo-500 dark:hover:bg-indigo-400";

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
    </div>
  );
}
