import { GraduationCap } from "lucide-react";

import { cn } from "@/app/utils";

export function Logo({ className }: { className?: string }) {
  return (
    <div className={cn("flex items-center gap-3", className)}>
      <div className="flex h-11 w-11 items-center justify-center rounded-2xl bg-brand text-white shadow-glow">
        <GraduationCap className="h-6 w-6" />
      </div>
      <div>
        <p className="font-heading text-lg font-semibold leading-none text-slate-950">
          Cursos Online
        </p>
        <p className="text-xs font-medium text-slate-500">
          Plataforma premium de aprendizaje
        </p>
      </div>
    </div>
  );
}
