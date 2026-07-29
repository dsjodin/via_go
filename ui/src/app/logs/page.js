"use client";

import { useEffect, useRef, useState } from "react";
import { Alert, Button, Card } from "@/components/ui";
import * as api from "@/lib/api";
import { useSession } from "@/lib/useSession";

const levelColour = {
  error: "text-red-400",
  fatal: "text-red-400",
  warning: "text-amber-400",
  info: "text-slate-300",
  debug: "text-slate-500",
};

export default function LogsPage() {
  const { loading } = useSession();
  const [lines, setLines] = useState([]);
  const [connected, setConnected] = useState(false);
  const [follow, setFollow] = useState(true);
  const bottom = useRef(null);
  const seq = useRef(0);

  useEffect(() => {
    if (loading) return;

    const ws = api.logSocket();

    ws.onopen = () => setConnected(true);
    ws.onclose = () => setConnected(false);
    ws.onerror = () => setConnected(false);

    ws.onmessage = (e) => {
      let entry;
      try {
        entry = JSON.parse(e.data);
      } catch {
        entry = { msg: e.data };
      }

      // Keep the buffer bounded; a busy imaging run produces a lot of lines
      // and the tab should not grow without limit.
      seq.current += 1;
      setLines((prev) => [...prev, { ...entry, id: seq.current }].slice(-2000));
    };

    return () => ws.close();
  }, [loading]);

  useEffect(() => {
    if (!follow || lines.length === 0) return;
    bottom.current?.scrollIntoView({ behavior: "smooth" });
  }, [lines.length, follow]);

  if (loading) return null;

  return (
    <div className="space-y-4">
      <header className="flex items-center justify-between">
        <div>
          <h1 className="text-xl font-semibold text-slate-100">Log</h1>
          <p className="text-sm text-slate-500">
            Live output from the DHCP, TFTP, boot and kickstart services.
          </p>
        </div>
        <div className="flex items-center gap-3">
          <span
            className={`text-xs ${connected ? "text-emerald-400" : "text-slate-500"}`}
          >
            {connected ? "● connected" : "○ disconnected"}
          </span>
          <Button variant="secondary" onClick={() => setFollow(!follow)}>
            {follow ? "Pause scroll" : "Follow"}
          </Button>
          <Button variant="secondary" onClick={() => setLines([])}>
            Clear
          </Button>
        </div>
      </header>

      {!connected && (
        <Alert kind="info">
          Not connected to the log stream. It reconnects when you reload the
          page.
        </Alert>
      )}

      <Card>
        <div className="max-h-[70vh] overflow-y-auto font-mono text-xs leading-relaxed">
          {lines.length === 0 && (
            <p className="py-6 text-center text-slate-500">
              Waiting for output…
            </p>
          )}
          {lines.map((l) => (
            <div key={l.id} className="flex gap-3 py-0.5">
              <span className="shrink-0 text-slate-600">
                {l.time ? new Date(l.time).toLocaleTimeString() : ""}
              </span>
              <span
                className={`shrink-0 uppercase ${levelColour[l.level] ?? "text-slate-500"}`}
              >
                {l.level ?? ""}
              </span>
              <span className="break-all text-slate-300">{l.msg ?? ""}</span>
            </div>
          ))}
          <div ref={bottom} />
        </div>
      </Card>
    </div>
  );
}
