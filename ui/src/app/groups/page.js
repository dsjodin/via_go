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
  Select,
  Table,
} from "@/components/ui";
import * as api from "@/lib/api";
import { useSession } from "@/lib/useSession";

const bootMethods = [
  { value: "pxe", label: "PXE (TFTP)" },
  { value: "http-dhcp", label: "UEFI HTTP boot" },
  { value: "https-dhcp", label: "UEFI HTTPS boot" },
];

export default function GroupsPage() {
  const { loading } = useSession();
  const [groups, setGroups] = useState([]);
  const [images, setImages] = useState([]);
  const [error, setError] = useState(null);
  const [editing, setEditing] = useState(null);

  const refresh = useCallback(async () => {
    try {
      const [g, i] = await Promise.all([api.listGroups(), api.listImages()]);
      setGroups(g || []);
      setImages(i || []);
      setError(null);
    } catch (err) {
      setError(err.message);
    }
  }, []);

  useEffect(() => {
    if (!loading) refresh();
  }, [loading, refresh]);

  if (loading) return null;

  const imageName = (id) =>
    images.find((i) => i.id === id)?.iso_image ?? (id ? `#${id}` : "—");

  async function remove(group) {
    if (!confirm(`Delete group ${group.name}?`)) return;
    try {
      await api.deleteGroup(group.id);
      refresh();
    } catch (err) {
      setError(err.message);
    }
  }

  return (
    <div className="space-y-4">
      <header className="flex items-center justify-between">
        <div>
          <h1 className="text-xl font-semibold text-slate-100">Groups</h1>
          <p className="text-sm text-slate-500">
            A group is the build profile a host installs with: image, network
            and kickstart options.
          </p>
        </div>
        <Button onClick={() => setEditing({})}>Add group</Button>
      </header>

      <Alert>{error}</Alert>

      <Card>
        <Table
          columns={["Name", "Image", "Network", "Boot", "VLAN", ""]}
          empty={groups.length === 0 ? "No groups yet." : null}
        >
          {groups.map((g) => (
            <tr key={g.id} className="hover:bg-slate-800/30">
              <td className="px-3 py-2 text-slate-200">{g.name}</td>
              <td className="px-3 py-2 text-slate-300">
                {imageName(g.image_id)}
              </td>
              <td className="px-3 py-2 font-mono text-xs text-slate-400">
                {g.gateway || "—"} / {g.netmask || "—"}
              </td>
              <td className="px-3 py-2 text-slate-300">
                {bootMethods.find((b) => b.value === g.bootmethod)?.label ?? (
                  <span className="text-amber-400">not set</span>
                )}
              </td>
              <td className="px-3 py-2 text-slate-400">{g.vlan || "—"}</td>
              <td className="px-3 py-2 text-right whitespace-nowrap">
                <Button variant="secondary" onClick={() => setEditing(g)}>
                  Edit
                </Button>{" "}
                <Button variant="danger" onClick={() => remove(g)}>
                  Delete
                </Button>
              </td>
            </tr>
          ))}
        </Table>
      </Card>

      {editing && (
        <GroupModal
          group={editing}
          images={images}
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

function GroupModal({ group, images, onClose, onSaved }) {
  const opts = (() => {
    try {
      return group.options ? JSON.parse(atob(group.options)) : {};
    } catch {
      return typeof group.options === "object" && group.options
        ? group.options
        : {};
    }
  })();

  const [form, setForm] = useState({
    name: group.name || "",
    image_id: group.image_id ?? "",
    bootmethod: group.bootmethod || "pxe",
    gateway: group.gateway || "",
    netmask: group.netmask || "",
    device: group.device || "",
    dns: group.dns || "",
    ntp: group.ntp || "",
    syslog: group.syslog || "",
    vlan: group.vlan || "",
    bootdisk: group.bootdisk || "",
    password: "",
  });
  const [options, setOptions] = useState({
    ssh: !!opts.ssh,
    erasedisks: !!opts.erasedisks,
    createvmfs: !!opts.createvmfs,
    suppressshellwarning: !!opts.suppressshellwarning,
  });
  const [error, setError] = useState(null);
  const [busy, setBusy] = useState(false);

  const set = (k) => (e) => setForm({ ...form, [k]: e.target.value });
  const setOpt = (k) => (e) =>
    setOptions({ ...options, [k]: e.target.checked });

  async function save(e) {
    e.preventDefault();
    setBusy(true);
    setError(null);

    const payload = {
      ...form,
      image_id: Number(form.image_id),
      options,
    };
    // An empty password on edit means "leave it alone"; the API treats it that
    // way, and sending an empty string would fail complexity validation.
    if (!payload.password) delete payload.password;

    try {
      if (group.id) {
        await api.updateGroup(group.id, payload);
      } else {
        await api.createGroup(payload);
      }
      onSaved();
    } catch (err) {
      setError(err.message);
    } finally {
      setBusy(false);
    }
  }

  return (
    <Modal title={group.id ? "Edit group" : "Add group"} onClose={onClose}>
      <form onSubmit={save} className="space-y-3">
        <Field label="Name">
          <Input value={form.name} onChange={set("name")} required />
        </Field>

        <div className="grid grid-cols-2 gap-3">
          <Field label="Image">
            <Select value={form.image_id} onChange={set("image_id")} required>
              <option value="">Select an image…</option>
              {images.map((i) => (
                <option key={i.id} value={i.id}>
                  {i.iso_image}
                </option>
              ))}
            </Select>
          </Field>

          <Field label="Boot method">
            <Select value={form.bootmethod} onChange={set("bootmethod")}>
              {bootMethods.map((b) => (
                <option key={b.value} value={b.value}>
                  {b.label}
                </option>
              ))}
            </Select>
          </Field>
        </div>

        <div className="grid grid-cols-2 gap-3">
          <Field label="Gateway">
            <Input
              value={form.gateway}
              onChange={set("gateway")}
              placeholder="192.168.1.1"
              required
            />
          </Field>
          <Field label="Netmask">
            <Input
              value={form.netmask}
              onChange={set("netmask")}
              placeholder="255.255.255.0"
              required
            />
          </Field>
        </div>

        <div className="grid grid-cols-2 gap-3">
          <Field label="Install NIC" hint="e.g. vmnic0">
            <Input value={form.device} onChange={set("device")} />
          </Field>
          <Field label="VLAN" hint="Leave empty for untagged">
            <Input value={form.vlan} onChange={set("vlan")} />
          </Field>
        </div>

        <div className="grid grid-cols-2 gap-3">
          <Field label="DNS" hint="Comma separated">
            <Input value={form.dns} onChange={set("dns")} />
          </Field>
          <Field label="NTP" hint="Comma separated">
            <Input value={form.ntp} onChange={set("ntp")} />
          </Field>
        </div>

        <Field label="Syslog">
          <Input
            value={form.syslog}
            onChange={set("syslog")}
            placeholder="udp://10.0.0.99:514"
          />
        </Field>

        <Field label="Boot disk" hint="Empty installs to the first local disk">
          <Input value={form.bootdisk} onChange={set("bootdisk")} />
        </Field>

        <Field
          label="ESXi root password"
          hint={
            group.id
              ? "Leave empty to keep the current password"
              : "Stored encrypted; written into the kickstart at install time"
          }
        >
          <Input
            type="password"
            autoComplete="new-password"
            value={form.password}
            onChange={set("password")}
            required={!group.id}
          />
        </Field>

        <fieldset className="space-y-2 rounded border border-slate-800 p-3">
          <legend className="px-1 text-xs tracking-wide text-slate-400 uppercase">
            Install options
          </legend>
          <Checkbox
            checked={options.ssh}
            onChange={setOpt("ssh")}
            label="Enable SSH"
          />
          <Checkbox
            checked={options.erasedisks}
            onChange={setOpt("erasedisks")}
            label="Erase all disks before installing"
          />
          <Checkbox
            checked={options.createvmfs}
            onChange={setOpt("createvmfs")}
            label="Create a VMFS datastore on the install disk"
          />
          <Checkbox
            checked={options.suppressshellwarning}
            onChange={setOpt("suppressshellwarning")}
            label="Suppress the shell warning"
          />
        </fieldset>

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
