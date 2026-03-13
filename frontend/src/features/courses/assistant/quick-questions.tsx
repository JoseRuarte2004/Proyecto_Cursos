import { Sparkles } from "lucide-react";

import { cn } from "@/app/utils";

export function QuickQuestions({
  prompts,
  selectedPrompt,
  disabled,
  onSelect,
}: {
  prompts: string[];
  selectedPrompt: string;
  disabled?: boolean;
  onSelect: (prompt: string) => void;
}) {
  return (
    <div>
      <div className="flex items-center gap-2 text-[11px] font-semibold uppercase tracking-[0.24em] text-slate-500">
        <Sparkles className="h-3.5 w-3.5 text-brand" />
        Preguntas rapidas
      </div>
      <div className="mt-3 space-y-2">
        {prompts.map((prompt) => {
          const active = selectedPrompt.trim() === prompt;
          return (
            <button
              key={prompt}
              type="button"
              disabled={disabled}
              onClick={() => onSelect(prompt)}
              className={cn(
                "block w-full rounded-[20px] border px-4 py-3 text-left text-sm leading-5 transition-all duration-200 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand/30 disabled:cursor-not-allowed disabled:opacity-60",
                active
                  ? "border-brand/20 bg-brand/10 text-brand-dark"
                  : "border-slate-200 bg-white/90 text-slate-700 hover:-translate-y-0.5 hover:border-brand/20 hover:text-slate-950",
              )}
            >
              {prompt}
            </button>
          );
        })}
      </div>
    </div>
  );
}
