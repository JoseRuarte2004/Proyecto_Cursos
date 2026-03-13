import { forwardRef, type SelectHTMLAttributes } from "react";

import { cn } from "@/app/utils";

export const Select = forwardRef<
  HTMLSelectElement,
  SelectHTMLAttributes<HTMLSelectElement>
>(({ className, children, ...props }, ref) => (
  <select
    ref={ref}
    className={cn(
      "h-12 w-full rounded-2xl border border-slate-200 bg-white/90 px-4 text-sm text-slate-900 outline-none transition focus:border-brand/50 focus:ring-4 focus:ring-brand/10",
      className,
    )}
    {...props}
  >
    {children}
  </select>
));

Select.displayName = "Select";
