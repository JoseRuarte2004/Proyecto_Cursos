import { type ReactNode } from "react";

export function PageIntro({
  eyebrow,
  title,
  description,
  actions,
}: {
  eyebrow?: string;
  title: string;
  description: string;
  actions?: ReactNode;
}) {
  return (
    <div className="flex flex-col gap-4 rounded-[32px] bg-white/80 p-6 shadow-card sm:p-8 lg:flex-row lg:items-end lg:justify-between">
      <div className="max-w-3xl">
        {eyebrow ? (
          <p className="text-xs font-semibold uppercase tracking-[0.28em] text-brand">
            {eyebrow}
          </p>
        ) : null}
        <h1 className="mt-2 font-heading text-3xl font-semibold text-slate-950 sm:text-4xl">
          {title}
        </h1>
        <p className="mt-3 text-sm leading-6 text-slate-600 sm:text-base">
          {description}
        </p>
      </div>
      {actions ? <div className="flex flex-wrap gap-3">{actions}</div> : null}
    </div>
  );
}
