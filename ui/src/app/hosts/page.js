"use client";

import { useCallback, useEffect, useState } from "react";
import {
  Alert,
  Button,
  Card,
  Checkbox,
  Field,
  Input,
  Modal,
  Progress,
  Select,
  Table,
} from "@/components/ui";
import * as api from "@/lib/api";
import { useSession } from "@/lib/useSession";

const blank = {
  hostname: "",
  domain: "",
  ip: "",
  mac: "",
  group_id: "",
  reimage: false,
};

export default function HostsPage() {
  const { loading } = useSession();
  const [hosts, setHosts] = useState([]);
  const [groups, setGroups] = useState([]);
  const [error, setError] = useState(null);
  const [editing, setEditing] = useState(null);

  const refresh = useCallback(async () => {
    try {
      const [h, g] = await Promise.all([api.listHosts(), api.listGroups()]);
      setHosts(h || []);
      setGroups(g || []);
      setError(null);
    } catch (err) {
      setError(err.message);
    }
  }, []);

  useEffect(() => {
    if (loading) return;
    refresh();

    // Installs move through the backend's progress stages over minutes, so the
    // table refreshes rather than showing whatever was true on page load.
    const t = setInterval(refresh, 5000);
    return () => clearInterval(t);
  }, [loading, refresh]);

  if (loading) return null;

  const groupName = (id) =>
    groups.find((g) => g.id === id)?.name ?? (id ? `#${id}` : "—");

  async function toggleReimage(host) {
    try {
      await api.updateHost(host.id, { reimage: !host.reimage });
      refresh();
    } catch (err) {
      setError(err.message);
    }
  }

  async function remove(host) {
    if (!confirm(`Delete ${host.hostname || host.mac}?`)) return;
    try {
      await api.deleteHost(host.id);
      refresh();
    } catch (err) {
      setError(err.message);
    }
  }

  return (
    <div className="space-y-4">
      <header className="flex items-center justify-between">
        <div>
          <h1 className="text-xl font-semibold text-slate-100">Hosts</h1>
          <p className="text-sm text-slate-500">
            A host is answered by DHCP only while it is flagged for re-imaging.
          </p>
        </div>
        <Button onClick={() => setEditing({ ...blank })}>Add host</Button>
      </header>

      <Alert>{error}</Alert>

      <Card>
        <Table
          columns={[
            "Hostname",
            "IP",
            "MAC",
            "Group",
            "Re-image",
            "Progress",
            "",
          ]}
          empty={hosts.length === 0 ? "No hosts yet." : null}
        >
          {hosts.map((h) => (
            <tr key={h.id} className="hover:bg-slate-800/30">
              <td className="px-3 py-2">
                <span className="text-slate-200">{h.hostname || "—"}</span>
                {h.domain && (
                  <span className="text-slate-500">.{h.domain}</span>
                )}
              </td>
              <td className="px-3 py-2 font-mono text-xs text-slate-300">
                {h.ip}
              </td>
              <td className="px-3 py-2 font-mono text-xs text-slate-400">
                {h.mac}
              </td>
              <td className="px-3 py-2 text-slate-300">
                {groupName(h.group_id)}
              </td>
              <td className="px-3 py-2">
                <Checkbox
                  checked={!!h.reimage}
                  onChange={() => toggleReimage(h)}
                  label={h.reimage ? "armed" : "off"}
                />
              </td>
              <td className="px-3 py-2">
                <Progress value={h.progress} text={h.progresstext} />
              </td>
              <td className="px-3 py-2 text-right whitespace-nowrap">
                <Button variant="secondary" onClick={() => setEditing(h)}>
                  Edit
                </Button>{" "}
                <Button variant="danger" onClick={() => remove(h)}>
                  Delete
                </Button>
              </td>
            </tr>
          ))}
        </Table>
      </Card>

      {editing && (
        <HostModal
          host={editing}
          groups={groups}
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

function HostModal({ host, groups, onClose, onSaved }) {
  const [form, setForm] = useState({
    hostname: host.hostname || "",
    domain: host.domain || "",
    ip: host.ip || "",
    mac: host.mac || "",
    group_id: host.group_id ?? "",
    reimage: !!host.reimage,
  });
  const [error, setError] = useState(null);
  const [busy, setBusy] = useState(false);

  const set = (k) => (e) =>
    setForm({
      ...form,
      [k]: e.target.type === "checkbox" ? e.target.checked : e.target.value,
    });

  async function save(e) {
    e.preventDefault();
    setBusy(true);
    setError(null);

    const payload = { ...form, group_id: Number(form.group_id) };

    try {
      if (host.id) {
        await api.updateHost(host.id, payload);
      } else {
        await api.createHost(payload);
      }
      onSaved();
    } catch (err) {
      setError(err.message);
    } finally {
      setBusy(false);
    }
  }

  return (
    <Modal title={host.id ? "Edit host" : "Add host"} onClose={onClose}>
      <form onSubmit={save} className="space-y-3">
        <div className="grid grid-cols-2 gap-3">
          <Field label="Hostname">
            <Input value={form.hostname} onChange={set("hostname")} required />
          </Field>
          <Field label="Domain">
            <Input value={form.domain} onChange={set("domain")} />
          </Field>
        </div>

        <Field
          label="MAC address"
          hint="The address that boots. DHCP matches on this."
        >
          <Input
            value={form.mac}
            onChange={set("mac")}
            placeholder="00:0c:29:00:00:01"
            required
          />
        </Field>

        <Field label="IP address" hint="Must be inside the group's network.">
          <Input
            value={form.ip}
            onChange={set("ip")}
            placeholder="192.168.1.50"
            required
          />
        </Field>

        <Field label="Group">
          <Select value={form.group_id} onChange={set("group_id")} required>
            <option value="">Select a group…</option>
            {groups.map((g) => (
              <option key={g.id} value={g.id}>
                {g.name}
              </option>
            ))}
          </Select>
        </Field>

        <Checkbox
          checked={form.reimage}
          onChange={set("reimage")}
          label="Flag for re-imaging — DHCP will answer this host and it will install on next boot"
        />

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
