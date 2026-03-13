import type { ReactNode } from "react";
import { motion } from "framer-motion";
import { AlertCircle, Inbox, LoaderCircle } from "lucide-react";

import { Card } from "@/components/ui/card";

export function LoaderScreen({ label }: { label: string }) {
  return (
    <div className="page-shell flex min-h-[60vh] items-center justify-center">
      <div className="flex flex-col items-center gap-4 text-center">
        <LoaderCircle className="h-10 w-10 animate-spin text-brand" />
        <p className="text-sm font-medium text-slate-600">{label}</p>
      </div>
    </div>
  );
}

export function LoadingSkeleton({
  lines = 3,
  className,
}: {
  lines?: number;
  className?: string;
}) {
  return (
    <Card className={className}>
      <div className="space-y-3 p-5">
        {Array.from({ length: lines }).map((_, index) => (
          <div
            key={index}
            className="h-4 animate-pulse rounded-full bg-slate-200/80"
            style={{ width: `${100 - index * 12}%` }}
          />
        ))}
      </div>
    </Card>
  );
}

export function EmptyState({
  title,
  description,
  action,
}: {
  title: string;
  description: string;
  action?: ReactNode;
}) {
  return (
    <Card className="p-8 text-center">
      <div className="mx-auto flex h-14 w-14 items-center justify-center rounded-2xl bg-slate-100 text-slate-500">
        <Inbox className="h-7 w-7" />
      </div>
      <h3 className="mt-4 font-heading text-xl font-semibold text-slate-950">
        {title}
      </h3>
      <p className="mx-auto mt-2 max-w-xl text-sm text-slate-600">{description}</p>
      {action ? <div className="mt-5">{action}</div> : null}
    </Card>
  );
}

export function ErrorState({
  title = "Algo salió mal",
  description,
  action,
}: {
  title?: string;
  description: string;
  action?: ReactNode;
}) {
  return (
    <Card className="border-rose-100 p-8 text-center">
      <div className="mx-auto flex h-14 w-14 items-center justify-center rounded-2xl bg-rose-100 text-rose-600">
        <AlertCircle className="h-7 w-7" />
      </div>
      <h3 className="mt-4 font-heading text-xl font-semibold text-slate-950">
        {title}
      </h3>
      <p className="mx-auto mt-2 max-w-xl text-sm text-slate-600">{description}</p>
      {action ? <div className="mt-5">{action}</div> : null}
    </Card>
  );
}

export function SectionMotion({ children }: { children: ReactNode }) {
  return (
    <motion.div
      initial={{ opacity: 0, y: 12 }}
      animate={{ opacity: 1, y: 0 }}
      transition={{ duration: 0.32, ease: "easeOut" }}
    >
      {children}
    </motion.div>
  );
}
