"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { Alert, Button, Card, Field, Input, Table } from "@/components/ui";
import * as api from "@/lib/api";
import { useSession } from "@/lib/useSession";

function humanSize(bytes) {
  if (!bytes) return "—";
  const units = ["B", "KB", "MB", "GB", "TB"];
  let n = bytes;
  let u = 0;
  while (n >= 1024 && u < units.length - 1) {
    n /= 1024;
    u++;
  }
  return `${n.toFixed(u === 0 ? 0 : 1)} ${units[u]}`;
}

export default function ImagesPage() {
  const { loading } = useSession();
  const [images, setImages] = useState([]);
  const [error, setError] = useState(null);
  const [progress, setProgress] = useState(null);
  const [description, setDescription] = useState("");
  const [hash, setHash] = useState("");
  const fileRef = useRef(null);

  const refresh = useCallback(async () => {
    try {
      setImages((await api.listImages()) || []);
      setError(null);
    } catch (err) {
      setError(err.message);
    }
  }, []);

  useEffect(() => {
    if (!loading) refresh();
  }, [loading, refresh]);

  if (loading) return null;

  async function upload(e) {
    e.preventDefault();
    const file = fileRef.current?.files?.[0];
    if (!file) return;

    setError(null);
    setProgress(0);

    try {
      await api.uploadImage({
        file,
        description,
        hash,
        onProgress: setProgress,
      });
      setDescription("");
      setHash("");
      if (fileRef.current) fileRef.current.value = "";
      refresh();
    } catch (err) {
      setError(err.message);
    } finally {
      setProgress(null);
    }
  }

  async function remove(image) {
    if (!confirm(`Delete ${image.iso_image} and its extracted files?`)) return;
    try {
      await api.deleteImage(image.id);
      refresh();
    } catch (err) {
      setError(err.message);
    }
  }

  return (
    <div className="space-y-4">
      <header>
        <h1 className="text-xl font-semibold text-slate-100">Images</h1>
        <p className="text-sm text-slate-500">
          ESXi ISOs are extracted on upload, and hosts boot the loader from
          inside the image assigned to their group.
        </p>
      </header>

      <Alert>{error}</Alert>

      <Card title="Upload an ISO">
        <form onSubmit={upload} className="space-y-3">
          <Field label="ISO file">
            <input
              ref={fileRef}
              type="file"
              accept=".iso"
              required
              className="w-full text-sm text-slate-300 file:mr-3 file:rounded file:border-0 file:bg-slate-700 file:px-3 file:py-1.5 file:text-sm file:text-slate-100 hover:file:bg-slate-600"
            />
          </Field>

          <div className="grid grid-cols-2 gap-3">
            <Field label="Description">
              <Input
                value={description}
                onChange={(e) => setDescription(e.target.value)}
                placeholder="ESXi 8.0U3"
              />
            </Field>
            <Field
              label="SHA-256"
              hint="Optional. Checked after upload; a mismatch is rejected."
            >
              <Input value={hash} onChange={(e) => setHash(e.target.value)} />
            </Field>
          </div>

          {progress !== null && (
            <div>
              <div className="h-1.5 w-full overflow-hidden rounded bg-slate-800">
                <div
                  className="h-full bg-sky-500 transition-all"
                  style={{ width: `${progress}%` }}
                />
              </div>
              <p className="mt-1 text-xs text-slate-500">
                {progress < 100
                  ? `Uploading ${progress}%`
                  : "Extracting — this takes a while for a full ISO"}
              </p>
            </div>
          )}

          <Button type="submit" disabled={progress !== null}>
            {progress !== null ? "Uploading…" : "Upload"}
          </Button>
        </form>
      </Card>

      <Card>
        <Table
          columns={["ISO", "Description", "Size", "Path", ""]}
          empty={images.length === 0 ? "No images uploaded yet." : null}
        >
          {images.map((i) => (
            <tr key={i.id} className="hover:bg-slate-800/30">
              <td className="px-3 py-2 text-slate-200">{i.iso_image}</td>
              <td className="px-3 py-2 text-slate-400">
                {i.description || "—"}
              </td>
              <td className="px-3 py-2 text-slate-400">{humanSize(i.size)}</td>
              <td className="px-3 py-2 font-mono text-xs text-slate-500">
                {i.path}
              </td>
              <td className="px-3 py-2 text-right">
                <Button variant="danger" onClick={() => remove(i)}>
                  Delete
                </Button>
              </td>
            </tr>
          ))}
        </Table>
      </Card>
    </div>
  );
}
