import { type PropsWithChildren } from "react";

import { cn } from "@/app/utils";

export function Badge({
  children,
  tone = "neutral",
}: PropsWithChildren<{
  tone?: "neutral" | "success" | "warning" | "brand" | "danger";
}>) {
  return (
    <span
      className={cn(
        "inline-flex items-center rounded-full px-3 py-1 text-xs font-semibold",
        tone === "neutral" && "bg-slate-100 text-slate-700",
        tone === "success" && "bg-emerald-100 text-emerald-700",
        tone === "warning" && "bg-amber-100 text-amber-700",
        tone === "brand" && "bg-brand-soft text-brand-dark",
        tone === "danger" && "bg-rose-100 text-rose-700",
      )}
    >
      {children}
    </span>
  );
}
