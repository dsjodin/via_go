"use client";

import { useRouter } from "next/navigation";
import { useEffect } from "react";

export default function Home() {
  const router = useRouter();

  // There is no separate dashboard: hosts is the screen this appliance is for.
  useEffect(() => {
    router.replace("/hosts");
  }, [router]);

  return null;
}
