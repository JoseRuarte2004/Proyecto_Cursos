import { forwardRef, type TextareaHTMLAttributes } from "react";

import { cn } from "@/app/utils";

export const Textarea = forwardRef<
  HTMLTextAreaElement,
  TextareaHTMLAttributes<HTMLTextAreaElement>
>(({ className, ...props }, ref) => (
  <textarea
    ref={ref}
    className={cn(
      "min-h-28 w-full rounded-2xl border border-slate-200 bg-white/90 px-4 py-3 text-sm text-slate-900 outline-none transition focus:border-brand/50 focus:ring-4 focus:ring-brand/10",
      className,
    )}
    {...props}
  />
));

Textarea.displayName = "Textarea";
