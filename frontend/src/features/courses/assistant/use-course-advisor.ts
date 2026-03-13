import { useMutation } from "@tanstack/react-query";
import { useState } from "react";

import { coursesApi } from "@/api/endpoints";
import { useSession } from "@/auth/session";
import type { CourseRecommendationMessage } from "@/api/types";

export const QUICK_PROMPTS = [
  "Quiero aprender a programar desde cero.",
  "Busco un curso de backend para empezar.",
  "Necesito una recomendacion simple para mi primer curso.",
];

type RecommendationRequest = {
  name?: string;
  question: string;
  history: CourseRecommendationMessage[];
};

export type CourseAdvisorController = {
  quickPrompts: string[];
  question: string;
  conversation: CourseRecommendationMessage[];
  pendingQuestion: string;
  errorMessage: string | null;
  isPending: boolean;
  canSubmit: boolean;
  hasConversation: boolean;
  setQuestion: (value: string) => void;
  selectPrompt: (prompt: string) => void;
  submitCurrentQuestion: () => void;
  resetConversation: () => void;
};

export function useCourseAdvisor(): CourseAdvisorController {
  const { user } = useSession();
  const [question, setQuestion] = useState(QUICK_PROMPTS[0]);
  const [conversation, setConversation] = useState<CourseRecommendationMessage[]>([]);

  const recommendMutation = useMutation({
    mutationFn: (input: RecommendationRequest) => coursesApi.recommend(input),
    onSuccess: (result, submitted) => {
      setConversation((current) => [
        ...current,
        { role: "user", content: submitted.question },
        { role: "assistant", content: result.answer },
      ]);
      setQuestion("");
    },
  });

  const trimmedQuestion = question.trim();
  const pendingQuestion =
    recommendMutation.isPending && recommendMutation.variables
      ? recommendMutation.variables.question
      : "";

  function submitCurrentQuestion() {
    if (!trimmedQuestion || recommendMutation.isPending) {
      return;
    }

    recommendMutation.mutate({
      name: user?.name,
      question: trimmedQuestion,
      history: conversation,
    });
  }

  function resetConversation() {
    if (recommendMutation.isPending) {
      return;
    }

    setConversation([]);
  }

  return {
    quickPrompts: QUICK_PROMPTS,
    question,
    conversation,
    pendingQuestion,
    errorMessage: recommendMutation.isError ? recommendMutation.error.message : null,
    isPending: recommendMutation.isPending,
    canSubmit: Boolean(trimmedQuestion) && !recommendMutation.isPending,
    hasConversation: conversation.length > 0,
    setQuestion,
    selectPrompt: setQuestion,
    submitCurrentQuestion,
    resetConversation,
  };
}
