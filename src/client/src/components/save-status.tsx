"use client";

import { Check } from "lucide-react";
import { useEffect, useState } from "react";

export type SaveState = "idle" | "loading" | "success" | "error";

// A save action always resolves to a state the reader can see: `loading`
// while it is in flight, then `success`, which fades back to `idle` on its
// own, or `error`, which stays until the next attempt.
export function useSaveStatus() {
  const [status, setStatus] = useState<SaveState>("idle");
  useEffect(() => {
    if (status !== "success") return;
    const timer = window.setTimeout(() => setStatus("idle"), 2000);
    return () => window.clearTimeout(timer);
  }, [status]);
  return [status, setStatus] as const;
}

export function SaveStatus({
  status,
  savingLabel,
  savedLabel,
}: {
  status: SaveState;
  savingLabel: string;
  savedLabel: string;
}) {
  if (status === "loading")
    return <output className="save-status">{savingLabel}</output>;
  if (status === "success")
    return (
      <output className="save-status save-status--success">
        <Check size={16} /> {savedLabel}
      </output>
    );
  return null;
}
