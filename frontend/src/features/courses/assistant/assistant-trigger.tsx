import { MessageSquareText, Sparkles } from "lucide-react";
import type { RefObject } from "react";

import { cn } from "@/app/utils";

export function AssistantTrigger({
  open,
  panelId,
  buttonRef,
  onClick,
}: {
  open: boolean;
  panelId: string;
  buttonRef: RefObject<HTMLButtonElement>;
  onClick: () => void;
}) {
  return (
    <button
      ref={buttonRef}
      type="button"
      aria-label={open ? "Cerrar asistente" : "Abrir asistente"}
      aria-controls={panelId}
      aria-expanded={open}
      aria-haspopup="dialog"
      onClick={onClick}
      className={cn(
        "fixed bottom-[calc(1rem+env(safe-area-inset-bottom))] right-4 z-[60] inline-flex items-center gap-3 rounded-full border px-4 py-3 text-left shadow-[0_18px_45px_rgba(15,23,42,0.16)] backdrop-blur-xl transition-all duration-200 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand/40 sm:right-6",
        open
          ? "pointer-events-none translate-y-2 scale-95 opacity-0"
          : "border-white/80 bg-white/95 text-slate-900 hover:-translate-y-0.5 hover:border-brand/20 hover:shadow-[0_22px_55px_rgba(15,23,42,0.2)]",
      )}
    >
      <span
        className={cn(
          "flex h-11 w-11 items-center justify-center rounded-full transition-colors",
          open ? "bg-white/12 text-white" : "bg-brand text-white shadow-glow",
        )}
      >
        {open ? <Sparkles className="h-5 w-5" /> : <MessageSquareText className="h-5 w-5" />}
      </span>
      <span className="min-w-0">
        <span className="block text-sm font-semibold leading-none">Asistente</span>
        <span
          className={cn(
            "mt-1 block text-xs leading-none",
            open ? "text-white/70" : "text-slate-500",
          )}
        >
          Catalogo real
        </span>
      </span>
    </button>
  );
}
