import {
  forwardRef,
  type ButtonHTMLAttributes,
  type PropsWithChildren,
} from "react";

import { cn } from "@/app/utils";

type ButtonProps = PropsWithChildren<
  ButtonHTMLAttributes<HTMLButtonElement> & {
    variant?: "primary" | "secondary" | "ghost" | "danger";
    size?: "sm" | "md" | "lg";
    loading?: boolean;
  }
>;

export const Button = forwardRef<HTMLButtonElement, ButtonProps>(
  (
    {
      className,
      children,
      variant = "primary",
      size = "md",
      loading,
      disabled,
      ...props
    },
    ref,
  ) => (
    <button
      ref={ref}
      className={cn(
        "inline-flex items-center justify-center gap-2 rounded-2xl font-semibold transition-all duration-200 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand/40 disabled:cursor-not-allowed disabled:opacity-60",
        variant === "primary" &&
          "bg-brand text-white shadow-glow hover:-translate-y-0.5 hover:bg-brand-dark",
        variant === "secondary" &&
          "border border-slate-200 bg-white text-slate-900 shadow-card hover:-translate-y-0.5 hover:border-brand/30",
        variant === "ghost" &&
          "bg-transparent text-slate-700 hover:bg-slate-100",
        variant === "danger" &&
          "bg-rose-600 text-white shadow-card hover:bg-rose-700",
        size === "sm" && "h-10 px-4 text-sm",
        size === "md" && "h-11 px-5 text-sm",
        size === "lg" && "h-12 px-6 text-base",
        className,
      )}
      disabled={disabled || loading}
      {...props}
    >
      {loading ? "Cargando..." : children}
    </button>
  ),
);

Button.displayName = "Button";
