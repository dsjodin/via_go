"use client";

import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { useEffect, useState } from "react";
import * as api from "@/lib/api";

const nav = [
  { name: "Hosts", href: "/hosts" },
  { name: "Groups", href: "/groups" },
  { name: "Images", href: "/images" },
  { name: "Users", href: "/users" },
  { name: "Log", href: "/logs" },
];

export default function Shell({ children }) {
  const pathname = usePathname();
  const router = useRouter();
  const [version, setVersion] = useState(null);

  // The login page is the whole screen: it is the one place reachable without
  // a session, so it gets no navigation around it.
  const bare = pathname === "/login";

  useEffect(() => {
    if (bare) return;
    api
      .version()
      .then(setVersion)
      .catch(() => {
        /* the session check on each page reports auth problems */
      });
  }, [bare]);

  if (bare) return children;

  async function signOut() {
    try {
      await api.logout();
    } finally {
      router.replace("/login");
    }
  }

  return (
    <div className="flex min-h-screen">
      <aside className="flex w-52 shrink-0 flex-col border-r border-slate-800 bg-slate-900/50">
        <div className="border-b border-slate-800 px-4 py-4">
          <span className="text-lg font-semibold tracking-tight text-slate-100">
            go-via
          </span>
          <span className="mt-0.5 block text-xs text-slate-500">
            ESXi imaging
          </span>
        </div>

        <nav className="flex-1 p-2">
          {nav.map((item) => {
            const active = pathname.startsWith(item.href);
            return (
              <Link
                key={item.href}
                href={item.href}
                className={`block rounded px-3 py-2 text-sm transition-colors ${
                  active
                    ? "bg-sky-600/15 font-medium text-sky-300"
                    : "text-slate-400 hover:bg-slate-800/60 hover:text-slate-200"
                }`}
              >
                {item.name}
              </Link>
            );
          })}
        </nav>

        <div className="border-t border-slate-800 p-3">
          <button
            type="button"
            onClick={signOut}
            className="w-full rounded px-3 py-2 text-left text-sm text-slate-400 hover:bg-slate-800/60 hover:text-slate-200"
          >
            Sign out
          </button>
          {version && (
            <p className="px-3 pt-2 text-xs text-slate-600">
              {version.version === "dev"
                ? "development build"
                : `v${version.version}`}
            </p>
          )}
        </div>
      </aside>

      <main className="flex-1 overflow-x-auto p-6">{children}</main>
    </div>
  );
}
