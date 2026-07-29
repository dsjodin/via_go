"use client";

import { useCallback, useEffect, useState } from "react";
import {
  Alert,
  Button,
  Card,
  Field,
  Input,
  Modal,
  Table,
} from "@/components/ui";
import * as api from "@/lib/api";
import { useSession } from "@/lib/useSession";

export default function UsersPage() {
  const { loading, session } = useSession();
  const [users, setUsers] = useState([]);
  const [error, setError] = useState(null);
  const [editing, setEditing] = useState(null);

  const refresh = useCallback(async () => {
    try {
      setUsers((await api.listUsers()) || []);
      setError(null);
    } catch (err) {
      setError(err.message);
    }
  }, []);

  useEffect(() => {
    if (!loading) refresh();
  }, [loading, refresh]);

  if (loading) return null;

  async function remove(user) {
    if (user.username === session?.username) {
      setError("you cannot delete the account you are signed in as");
      return;
    }
    if (!confirm(`Delete user ${user.username}?`)) return;
    try {
      await api.deleteUser(user.id);
      refresh();
    } catch (err) {
      setError(err.message);
    }
  }

  return (
    <div className="space-y-4">
      <header className="flex items-center justify-between">
        <div>
          <h1 className="text-xl font-semibold text-slate-100">Users</h1>
          <p className="text-sm text-slate-500">
            Accounts that can sign in to this console and call the API.
          </p>
        </div>
        <Button onClick={() => setEditing({})}>Add user</Button>
      </header>

      <Alert>{error}</Alert>

      <Card>
        <Table
          columns={["Username", "Email", "Comment", ""]}
          empty={users.length === 0 ? "No users." : null}
        >
          {users.map((u) => (
            <tr key={u.id} className="hover:bg-slate-800/30">
              <td className="px-3 py-2 text-slate-200">
                {u.username}
                {u.username === session?.username && (
                  <span className="ml-2 text-xs text-slate-500">(you)</span>
                )}
                {u.must_change_password && (
                  <span className="ml-2 rounded bg-amber-950 px-1.5 py-0.5 text-xs text-amber-300">
                    default password
                  </span>
                )}
              </td>
              <td className="px-3 py-2 text-slate-400">{u.email || "—"}</td>
              <td className="px-3 py-2 text-slate-400">{u.comment || "—"}</td>
              <td className="px-3 py-2 text-right whitespace-nowrap">
                <Button variant="secondary" onClick={() => setEditing(u)}>
                  Edit
                </Button>{" "}
                <Button variant="danger" onClick={() => remove(u)}>
                  Delete
                </Button>
              </td>
            </tr>
          ))}
        </Table>
      </Card>

      {editing && (
        <UserModal
          user={editing}
          isSelf={editing.username === session?.username}
          onClose={() => setEditing(null)}
          onSaved={() => {
            setEditing(null);
            refresh();
          }}
        />
      )}
    </div>
  );
}

function UserModal({ user, isSelf, onClose, onSaved }) {
  const [form, setForm] = useState({
    username: user.username || "",
    email: user.email || "",
    comment: user.comment || "",
    password: "",
  });
  const [error, setError] = useState(null);
  const [busy, setBusy] = useState(false);

  const set = (k) => (e) => setForm({ ...form, [k]: e.target.value });

  async function save(e) {
    e.preventDefault();
    setBusy(true);
    setError(null);

    const payload = { ...form };
    if (!payload.password) delete payload.password;

    try {
      if (user.id) {
        await api.updateUser(user.id, payload);
      } else {
        await api.createUser(payload);
      }
      onSaved();
    } catch (err) {
      setError(err.message);
    } finally {
      setBusy(false);
    }
  }

  return (
    <Modal title={user.id ? "Edit user" : "Add user"} onClose={onClose}>
      <form onSubmit={save} className="space-y-3">
        <Field label="Username">
          <Input
            value={form.username}
            onChange={set("username")}
            disabled={!!user.id}
            required
          />
        </Field>

        <Field label="Email">
          <Input type="email" value={form.email} onChange={set("email")} />
        </Field>

        <Field label="Comment">
          <Input value={form.comment} onChange={set("comment")} />
        </Field>

        <Field
          label="Password"
          hint={
            user.id
              ? "Leave empty to keep the current password. Changing it signs out that account's other sessions."
              : undefined
          }
        >
          <Input
            type="password"
            autoComplete="new-password"
            value={form.password}
            onChange={set("password")}
            required={!user.id}
          />
        </Field>

        {isSelf && form.password && (
          <Alert kind="warn">
            This is the account you are signed in as. Changing its password ends
            this session and you will be asked to sign in again.
          </Alert>
        )}

        <Alert>{error}</Alert>

        <div className="flex justify-end gap-2 pt-1">
          <Button variant="secondary" type="button" onClick={onClose}>
            Cancel
          </Button>
          <Button type="submit" disabled={busy}>
            {busy ? "Saving…" : "Save"}
          </Button>
        </div>
      </form>
    </Modal>
  );
}
