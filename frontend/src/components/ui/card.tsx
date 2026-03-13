import { type HTMLAttributes } from "react";

import { cn } from "@/app/utils";

export function Card({
  className,
  ...props
}: HTMLAttributes<HTMLDivElement>) {
  return (
    <div
      className={cn(
        "glass-panel rounded-[28px] border border-white/70 shadow-card",
        className,
      )}
      {...props}
    />
  );
}
