import { useEffect, useRef, useState, type FormEvent, type RefObject } from "react";
import { AnimatePresence, motion } from "framer-motion";
import { ArrowDown, ArrowUp, Minimize2, Sparkles, WandSparkles, X } from "lucide-react";

import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";

import { AssistantConversation } from "@/features/courses/assistant/assistant-conversation";
import { QuickQuestions } from "@/features/courses/assistant/quick-questions";
import type { CourseAdvisorController } from "@/features/courses/assistant/use-course-advisor";

export function AssistantPanel({
  open,
  panelId,
  panelRef,
  inputRef,
  controller,
  onClose,
}: {
  open: boolean;
  panelId: string;
  panelRef: RefObject<HTMLDivElement>;
  inputRef: RefObject<HTMLInputElement>;
  controller: CourseAdvisorController;
  onClose: () => void;
}) {
  const [showScrollToBottom, setShowScrollToBottom] = useState(false);
  const scrollRef = useRef<HTMLDivElement>(null);

  function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    controller.submitCurrentQuestion();
  }

  function syncScrollState() {
    if (!scrollRef.current) {
      return;
    }
    const viewport = scrollRef.current;

    const distanceToBottom =
      viewport.scrollHeight - viewport.scrollTop - viewport.clientHeight;

    setShowScrollToBottom(distanceToBottom > 64);
  }

  function scrollToBottom() {
    if (!scrollRef.current) {
      return;
    }
    scrollRef.current.scrollTo({
      top: scrollRef.current.scrollHeight,
      behavior: "smooth",
    });
  }

  useEffect(() => {
    syncScrollState();
  }, [controller.conversation.length, controller.pendingQuestion, controller.isPending]);

  useEffect(() => {
    if (!open || !scrollRef.current) {
      return;
    }

    scrollRef.current.scrollTo({
      top: scrollRef.current.scrollHeight,
      behavior: "smooth",
    });
  }, [open, controller.conversation.length, controller.pendingQuestion, controller.isPending]);

  return (
    <AnimatePresence>
      {open ? (
        <>
          <motion.div
            aria-hidden="true"
            className="fixed inset-0 z-40 bg-slate-950/8 backdrop-blur-[1px] sm:bg-slate-950/5"
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            exit={{ opacity: 0 }}
          />
          <motion.section
            ref={panelRef}
            id={panelId}
            role="dialog"
            aria-modal="false"
            aria-labelledby={`${panelId}-title`}
            aria-describedby={`${panelId}-description`}
            className="fixed inset-x-2 top-[max(4.75rem,calc(env(safe-area-inset-top)+0.5rem))] bottom-[calc(0.75rem+env(safe-area-inset-bottom))] z-50 w-auto sm:inset-x-auto sm:right-6 sm:top-20 sm:bottom-6 sm:w-[24rem] lg:w-[25.5rem]"
            initial={{ opacity: 0, y: 18, scale: 0.96 }}
            animate={{ opacity: 1, y: 0, scale: 1 }}
            exit={{ opacity: 0, y: 18, scale: 0.96 }}
            transition={{ duration: 0.22, ease: "easeOut" }}
          >
            <Card className="grid h-full min-h-0 grid-rows-[auto,minmax(0,1fr),auto] overflow-hidden rounded-[28px] border border-white/85 bg-[linear-gradient(180deg,rgba(255,255,255,0.98),rgba(248,250,252,0.98))] shadow-[0_30px_80px_rgba(15,23,42,0.18)]">
              <div className="border-b border-slate-200/80 px-4 py-3 sm:px-5">
                <div className="flex items-start justify-between gap-4">
                  <div className="min-w-0 flex-1">
                    <div className="flex items-center gap-3">
                      <div className="flex h-9 w-9 items-center justify-center rounded-full bg-brand text-white shadow-glow">
                        <WandSparkles className="h-4 w-4" />
                      </div>
                      <div className="min-w-0">
                        <div className="flex items-center gap-2">
                          <h2
                            id={`${panelId}-title`}
                            className="font-heading text-xl font-semibold text-slate-950"
                          >
                            Asistente
                          </h2>
                          <span className="rounded-full bg-slate-100 px-2 py-0.5 text-[11px] font-semibold text-slate-500">
                            Beta
                          </span>
                        </div>
                        <p
                          id={`${panelId}-description`}
                          className="mt-1 text-xs leading-5 text-slate-500"
                        >
                          Te recomienda cursos segun el catalogo real publicado.
                        </p>
                      </div>
                    </div>
                  </div>

                  <div className="flex items-center gap-1">
                    <Button
                      type="button"
                      variant="ghost"
                      size="sm"
                      onClick={onClose}
                      aria-label="Minimizar asistente"
                    >
                      <Minimize2 className="h-4 w-4" />
                    </Button>
                    <Button
                      type="button"
                      variant="ghost"
                      size="sm"
                      onClick={onClose}
                      aria-label="Cerrar asistente"
                    >
                      <X className="h-4 w-4" />
                    </Button>
                  </div>
                </div>
              </div>

              <div className="relative min-h-0 overflow-hidden bg-[linear-gradient(180deg,rgba(248,250,252,0.45),rgba(255,255,255,0.75))]">
                <div
                  ref={scrollRef}
                  data-assistant-scroll
                  onScroll={syncScrollState}
                  className="h-full min-h-0 overflow-y-auto overscroll-contain px-4 py-4 sm:px-5"
                >
                  {!controller.hasConversation ? (
                    <div className="mb-4 rounded-[24px] border border-slate-200/90 bg-white/88 p-4 shadow-[0_14px_35px_rgba(15,23,42,0.05)]">
                      <QuickQuestions
                        prompts={controller.quickPrompts}
                        selectedPrompt={controller.question}
                        disabled={controller.isPending}
                        onSelect={controller.selectPrompt}
                      />
                    </div>
                  ) : null}

                  <AssistantConversation
                    conversation={controller.conversation}
                    pendingQuestion={controller.pendingQuestion}
                    isPending={controller.isPending}
                    errorMessage={controller.errorMessage}
                  />

                  <div className="h-3" />
                </div>

                {showScrollToBottom ? (
                  <button
                    type="button"
                    aria-label="Ir al ultimo mensaje"
                    onClick={scrollToBottom}
                    className="absolute bottom-4 right-4 flex h-11 w-11 items-center justify-center rounded-full border border-slate-200 bg-white/95 text-slate-700 shadow-[0_14px_28px_rgba(15,23,42,0.14)] transition-all duration-200 hover:-translate-y-0.5 hover:border-brand/20 hover:text-slate-950 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand/40"
                  >
                    <ArrowDown className="h-4 w-4" />
                  </button>
                ) : null}
              </div>

              <div className="border-t border-slate-200/80 bg-white/98 px-4 py-3 sm:px-5">
                <p className="mb-3 text-xs leading-5 text-slate-500">
                  La respuesta se basa solo en cursos publicados.
                </p>
                <form onSubmit={handleSubmit}>
                  <div className="flex items-center gap-2 rounded-[22px] border border-slate-200 bg-slate-50 px-3 py-2 shadow-[inset_0_1px_0_rgba(255,255,255,0.9)]">
                    <Sparkles className="h-4 w-4 shrink-0 text-slate-400" />
                    <Input
                      ref={inputRef}
                      id={`${panelId}-question`}
                      aria-label="Pregunta para el asistente"
                      value={controller.question}
                      onChange={(event) => controller.setQuestion(event.target.value)}
                      placeholder="Preguntale al asistente..."
                      className="h-11 border-0 bg-transparent px-0 text-sm shadow-none focus:ring-0"
                    />
                    <button
                      type="submit"
                      aria-label="Enviar consulta"
                      disabled={!controller.canSubmit}
                      className="inline-flex h-10 w-10 shrink-0 items-center justify-center rounded-full bg-slate-200 text-slate-500 transition-all duration-200 disabled:cursor-not-allowed disabled:opacity-60 enabled:bg-brand enabled:text-white enabled:shadow-glow enabled:hover:-translate-y-0.5"
                    >
                      <ArrowUp className="h-4 w-4" />
                    </button>
                  </div>
                </form>
              </div>
            </Card>
          </motion.section>
        </>
      ) : null}
    </AnimatePresence>
  );
}
