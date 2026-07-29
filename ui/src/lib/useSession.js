"use client";

import { useRouter } from "next/navigation";
import { useEffect, useState } from "react";
import * as api from "./api";

// useSession resolves who is logged in, sending anyone who is not to the login
// page. Every screen except /login depends on it.
export function useSession() {
  const router = useRouter();
  const [state, setState] = useState({ loading: true, session: null });

  useEffect(() => {
    let cancelled = false;

    api
      .session()
      .then((s) => {
        if (cancelled) return;
        setState({ loading: false, session: s });

        // An account still on the password it was seeded with is sent to
        // change it before anything else.
        if (s.must_change_password) router.replace("/login?change=1");
      })
      .catch(() => {
        if (!cancelled) router.replace("/login");
      });

    return () => {
      cancelled = true;
    };
  }, [router]);

  return state;
}
