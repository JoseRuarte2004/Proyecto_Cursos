import { MessageSquareText } from "lucide-react";

import type { CourseRecommendationMessage } from "@/api/types";

export function AssistantConversation({
  conversation,
  pendingQuestion,
  isPending,
  errorMessage,
}: {
  conversation: CourseRecommendationMessage[];
  pendingQuestion: string;
  isPending: boolean;
  errorMessage: string | null;
}) {
  const hasMessages = conversation.length > 0 || isPending || Boolean(errorMessage);

  if (!hasMessages) {
    return (
      <div className="rounded-[24px] border border-dashed border-slate-200 bg-white/80 px-4 py-5 text-sm leading-6 text-slate-500">
        Hace una pregunta concreta y el asistente te responde en base al catalogo
        publicado.
      </div>
    );
  }

  return (
    <div className="space-y-3">
      {conversation.map((message, index) => (
        <div
          key={`${message.role}-${index}`}
          className={
            message.role === "user"
              ? "ml-auto max-w-[88%] rounded-[22px] border border-brand/10 bg-brand px-4 py-3 text-sm leading-6 text-white shadow-card"
              : "max-w-[88%] rounded-[22px] border border-white/80 bg-white px-4 py-4 text-sm leading-6 text-slate-700 shadow-card"
          }
        >
          <p
            className={
              message.role === "user"
                ? "mb-1 text-[11px] font-semibold uppercase tracking-[0.18em] text-white/70"
                : "mb-1 flex items-center gap-2 text-[11px] font-semibold uppercase tracking-[0.18em] text-slate-400"
            }
          >
            {message.role === "assistant" ? (
              <MessageSquareText className="h-3.5 w-3.5 text-brand" />
            ) : null}
            {message.role === "user" ? "Vos" : "Asistente"}
          </p>
          <p className="whitespace-pre-wrap">{message.content}</p>
        </div>
      ))}

      {isPending && pendingQuestion ? (
        <>
          <div className="ml-auto max-w-[88%] rounded-[22px] border border-brand/10 bg-brand px-4 py-3 text-sm leading-6 text-white shadow-card">
            <p className="mb-1 text-[11px] font-semibold uppercase tracking-[0.18em] text-white/70">
              Vos
            </p>
            <p className="whitespace-pre-wrap">{pendingQuestion}</p>
          </div>
          <div className="max-w-[88%] rounded-[22px] border border-white/80 bg-white px-4 py-4 shadow-card">
            <p className="mb-3 flex items-center gap-2 text-[11px] font-semibold uppercase tracking-[0.18em] text-slate-400">
              <MessageSquareText className="h-3.5 w-3.5 text-brand" />
              Asistente
            </p>
            <div className="space-y-3">
              <div className="h-4 animate-pulse rounded-full bg-slate-200" />
              <div className="h-4 w-11/12 animate-pulse rounded-full bg-slate-200" />
              <div className="h-4 w-8/12 animate-pulse rounded-full bg-slate-200" />
            </div>
          </div>
        </>
      ) : null}

      {errorMessage ? (
        <div className="rounded-[20px] border border-rose-100 bg-rose-50 px-4 py-4 text-sm text-rose-700">
          {errorMessage}
        </div>
      ) : null}
    </div>
  );
}
