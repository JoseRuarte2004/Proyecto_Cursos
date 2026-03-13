import { useQuery } from "@tanstack/react-query";
import { ListChecks } from "lucide-react";
import { useEffect, useMemo, useState } from "react";
import { Link, useParams } from "react-router-dom";

import { LAST_LESSON_KEY_PREFIX } from "@/app/constants";
import { coursesApi, contentApi } from "@/api/endpoints";
import { ApiError } from "@/api/client";
import { readJSON } from "@/app/utils";
import { useSession } from "@/auth/session";
import { Button } from "@/components/ui/button";
import { PageIntro } from "@/components/shared/page-intro";
import {
  EmptyState,
  ErrorState,
  LoaderScreen,
} from "@/components/shared/feedback";
import {
  LessonProgressCard,
  LessonSidebar,
  VideoPlayer,
} from "@/features/courses/components";
import { useCourseProgress } from "@/features/courses/progress";
import { CourseChatPanel } from "@/features/chat/course-chat-panel";

export function ClassroomPage() {
  const { courseId = "" } = useParams();
  const { user } = useSession();
  const [selectedLessonId, setSelectedLessonId] = useState<string | null>(null);

  const lessonsQuery = useQuery({
    queryKey: ["lessons", courseId],
    queryFn: () => contentApi.listLessons(courseId),
    enabled: Boolean(courseId),
  });

  const courseQuery = useQuery({
    queryKey: ["course-preview", courseId],
    queryFn: () => coursesApi.getPublished(courseId),
    enabled: Boolean(courseId),
    retry: 0,
  });

  const lessons = lessonsQuery.data ?? [];
  const isStudent = user?.role === "student";
  const isTeacher = user?.role === "teacher";
  const isAdmin = user?.role === "admin";
  const manageLessonsLink = isTeacher
    ? `/teacher/courses/${courseId}/lessons`
    : isAdmin
      ? `/admin/courses/${courseId}/lessons`
      : null;
  const fallbackLink = isTeacher ? "/teacher/my-courses" : "/me/courses";
  const progress = useCourseProgress(courseId, isStudent ? user?.id ?? null : null);

  useEffect(() => {
    if (!lessons.length) {
      return;
    }

    const saved = readJSON<string | null>(
      `${LAST_LESSON_KEY_PREFIX}${courseId}`,
      null,
    );
    const next =
      lessons.find((lesson) => lesson.id === saved)?.id ?? lessons[0]?.id ?? null;
    setSelectedLessonId((current) => current ?? next);
  }, [courseId, lessons]);

  useEffect(() => {
    if (!selectedLessonId) {
      return;
    }
    localStorage.setItem(
      `${LAST_LESSON_KEY_PREFIX}${courseId}`,
      JSON.stringify(selectedLessonId),
    );
  }, [courseId, selectedLessonId]);

  const selectedLesson = useMemo(
    () => lessons.find((lesson) => lesson.id === selectedLessonId) ?? lessons[0],
    [lessons, selectedLessonId],
  );

  if (lessonsQuery.isLoading) {
    return <LoaderScreen label="Preparando el aula..." />;
  }

  if (lessonsQuery.isError) {
    const error = lessonsQuery.error;
    if (error instanceof ApiError && error.status === 403) {
      return (
        <div className="page-shell">
          <ErrorState
            title="No tenes acceso a este curso"
            description={
              isTeacher
                ? "Tu usuario no esta asignado a este curso. Volve al panel docente para abrir uno de tus cursos."
                : "Tu sesion no tiene permisos para ver estas clases. Si sos alumno, verifica que la inscripcion ya este activa."
            }
            action={
              <Link to={fallbackLink}>
                <Button>
                  {isTeacher ? "Ir a mis cursos docentes" : "Ir a mis cursos"}
                </Button>
              </Link>
            }
          />
        </div>
      );
    }

    return (
      <div className="page-shell">
        <ErrorState description={lessonsQuery.error.message} />
      </div>
    );
  }

  if (!lessons.length || !selectedLesson) {
    return (
      <div className="page-shell">
        <EmptyState
          title={
            manageLessonsLink
              ? "Todavia no hay videos cargados"
              : "Todavia no hay clases cargadas"
          }
          description={
            manageLessonsLink
              ? "Desde este curso ya podes abrir el panel de lessons para agregar, editar o borrar videos."
              : "Cuando el equipo docente cargue lessons vas a verlas aca en una navegacion tipo classroom."
          }
          action={
            manageLessonsLink ? (
              <Link to={manageLessonsLink}>
                <Button>
                  <ListChecks className="h-4 w-4" />
                  Gestionar videos
                </Button>
              </Link>
            ) : undefined
          }
        />
      </div>
    );
  }

  return (
    <div className="page-shell space-y-8">
      <PageIntro
        eyebrow="Classroom"
        title={courseQuery.data?.title ?? "Aula del curso"}
        description={
          courseQuery.data?.description ??
          "Recorre las lecciones, retoma donde lo dejaste y reproduce el contenido desde una sola pantalla."
        }
        actions={
          manageLessonsLink ? (
            <Link to={manageLessonsLink}>
              <Button variant="secondary">
                <ListChecks className="h-4 w-4" />
                Gestionar videos
              </Button>
            </Link>
          ) : undefined
        }
      />
      <div className="grid gap-6 xl:grid-cols-[320px_minmax(0,1fr)] 2xl:grid-cols-[320px_minmax(0,1fr)_360px]">
        <LessonSidebar
          lessons={lessons}
          activeLessonId={selectedLesson.id}
          completedLessonIds={isStudent ? progress.completedLessonIds : undefined}
          onSelect={(lesson) => setSelectedLessonId(lesson.id)}
        />
        <div className="space-y-6">
          {isStudent ? (
            <LessonProgressCard
              lessons={lessons}
              completedLessonIds={progress.completedLessonIds}
              currentLesson={selectedLesson}
              onToggleCurrentLesson={() =>
                progress.toggleLessonCompleted(selectedLesson.id)
              }
            />
          ) : null}
          <VideoPlayer lesson={selectedLesson} />
        </div>
        <div className="xl:col-span-2 2xl:col-span-1">
          <CourseChatPanel courseId={courseId} />
        </div>
      </div>
    </div>
  );
}
