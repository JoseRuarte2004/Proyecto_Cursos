import { useEffect, useMemo, useState } from "react";

import { LESSON_PROGRESS_KEY_PREFIX } from "@/app/constants";
import { persistJSON, readJSON } from "@/app/utils";

type StoredLessonProgress = {
  completedLessonIds: string[];
  updatedAt: string | null;
};

const EMPTY_PROGRESS: StoredLessonProgress = {
  completedLessonIds: [],
  updatedAt: null,
};

function buildProgressKey(courseId: string, userId: string) {
  return `${LESSON_PROGRESS_KEY_PREFIX}${userId}:${courseId}`;
}

function normalizeProgress(value: StoredLessonProgress): StoredLessonProgress {
  return {
    completedLessonIds: Array.from(
      new Set((value.completedLessonIds ?? []).filter(Boolean)),
    ),
    updatedAt: value.updatedAt ?? null,
  };
}

export function readCourseProgress(courseId: string, userId: string) {
  return normalizeProgress(
    readJSON<StoredLessonProgress>(
      buildProgressKey(courseId, userId),
      EMPTY_PROGRESS,
    ),
  );
}

export function useCourseProgress(courseId: string, userId: string | null) {
  const [progress, setProgress] = useState<StoredLessonProgress>(EMPTY_PROGRESS);

  useEffect(() => {
    if (!courseId || !userId) {
      setProgress(EMPTY_PROGRESS);
      return;
    }

    setProgress(readCourseProgress(courseId, userId));
  }, [courseId, userId]);

  const persist = (next: StoredLessonProgress) => {
    if (!courseId || !userId) {
      setProgress(EMPTY_PROGRESS);
      return;
    }

    const normalized = normalizeProgress(next);
    persistJSON(buildProgressKey(courseId, userId), normalized);
    setProgress(normalized);
  };

  const setLessonCompleted = (lessonId: string, completed: boolean) => {
    if (!lessonId) {
      return;
    }

    const completedLessonIds = completed
      ? Array.from(new Set([...progress.completedLessonIds, lessonId]))
      : progress.completedLessonIds.filter((id) => id !== lessonId);

    persist({
      completedLessonIds,
      updatedAt: new Date().toISOString(),
    });
  };

  const value = useMemo(
    () => ({
      completedLessonIds: progress.completedLessonIds,
      completedCount: progress.completedLessonIds.length,
      updatedAt: progress.updatedAt,
      isCompleted: (lessonId: string) =>
        progress.completedLessonIds.includes(lessonId),
      setLessonCompleted,
      toggleLessonCompleted: (lessonId: string) =>
        setLessonCompleted(lessonId, !progress.completedLessonIds.includes(lessonId)),
    }),
    [progress],
  );

  return value;
}
