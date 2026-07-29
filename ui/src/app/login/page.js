"use client";

import { useRouter, useSearchParams } from "next/navigation";
import { Suspense, useEffect, useState } from "react";
import { Alert, Button, Field, Input } from "@/components/ui";
import * as api from "@/lib/api";

function LoginForm() {
  const router = useRouter();
  const params = useSearchParams();

  // The session endpoint sends anyone still on the seeded password here with
  // ?change=1 rather than letting them into the console first.
  const [mode, setMode] = useState(params.get("change") ? "change" : "login");
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [next, setNext] = useState("");
  const [confirm, setConfirm] = useState("");
  const [error, setError] = useState(null);
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    if (mode !== "change") return;
    api
      .session()
      .then((s) => setUsername(s.username))
      .catch(() => setMode("login"));
  }, [mode]);

  async function signIn(e) {
    e.preventDefault();
    setBusy(true);
    setError(null);

    try {
      const s = await api.login(username, password);
      if (s.must_change_password) {
        setMode("change");
        setPassword("");
      } else {
        router.replace("/hosts");
      }
    } catch (err) {
      setError(err.message);
    } finally {
      setBusy(false);
    }
  }

  async function changePassword(e) {
    e.preventDefault();

    if (next !== confirm) {
      setError("the two passwords do not match");
      return;
    }

    setBusy(true);
    setError(null);

    try {
      const users = await api.listUsers();
      const me = users.find((u) => u.username === username);
      if (!me) throw new Error("could not find the account to update");

      await api.updateUser(me.id, { password: next });

      // Changing a password invalidates its sessions, this one included, so
      // sign in again with the new one rather than pretending to be logged in.
      await api.login(username, next);
      router.replace("/hosts");
    } catch (err) {
      setError(err.message);
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="flex min-h-screen items-center justify-center bg-slate-950 p-6">
      <div className="w-full max-w-sm">
        <div className="mb-6 text-center">
          <h1 className="text-2xl font-semibold tracking-tight text-slate-100">
            go-via
          </h1>
          <p className="mt-1 text-sm text-slate-500">ESXi imaging appliance</p>
        </div>

        <div className="rounded-lg border border-slate-800 bg-slate-900/60 p-5">
          {mode === "login"
            ? <form onSubmit={signIn} className="space-y-4">
                <Field label="Username">
                  <Input
                    autoFocus
                    autoComplete="username"
                    value={username}
                    onChange={(e) => setUsername(e.target.value)}
                    required
                  />
                </Field>

                <Field label="Password">
                  <Input
                    type="password"
                    autoComplete="current-password"
                    value={password}
                    onChange={(e) => setPassword(e.target.value)}
                    required
                  />
                </Field>

                <Alert>{error}</Alert>

                <Button type="submit" disabled={busy} className="w-full">
                  {busy ? "Signing in…" : "Sign in"}
                </Button>
              </form>
            : <form onSubmit={changePassword} className="space-y-4">
                <Alert kind="warn">
                  This account is still using the password it was installed
                  with. go-via stores and hands out ESXi root passwords, so
                  choose a new one before continuing.
                </Alert>

                <Field label="New password">
                  <Input
                    autoFocus
                    type="password"
                    autoComplete="new-password"
                    value={next}
                    onChange={(e) => setNext(e.target.value)}
                    required
                  />
                </Field>

                <Field label="Confirm new password">
                  <Input
                    type="password"
                    autoComplete="new-password"
                    value={confirm}
                    onChange={(e) => setConfirm(e.target.value)}
                    required
                  />
                </Field>

                <Alert>{error}</Alert>

                <Button type="submit" disabled={busy} className="w-full">
                  {busy ? "Saving…" : "Set password"}
                </Button>
              </form>}
        </div>
      </div>
    </div>
  );
}

export default function LoginPage() {
  // useSearchParams needs a suspense boundary in a statically exported app.
  return (
    <Suspense>
      <LoginForm />
    </Suspense>
  );
}
