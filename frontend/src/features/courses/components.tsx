import type { ReactNode } from "react";
import { motion } from "framer-motion";
import {
  ArrowRight,
  CheckCircle2,
  Circle,
  Clock3,
  Download,
  FileText,
  Image as ImageIcon,
  PlayCircle,
  Users,
  Video,
} from "lucide-react";
import { Link } from "react-router-dom";

import { buildAssetURL } from "@/api/client";
import { cn, formatCurrency, getVideoEmbed } from "@/app/utils";
import type { Course, Lesson } from "@/api/types";
import { useSession } from "@/auth/session";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";

type CourseCardData =
  | Course
  | {
      id: string;
      title: string;
      category: string;
      imageUrl?: string | null;
      price: number;
      currency: string;
      status?: string;
      description?: string;
      capacity?: number;
    };

export function CourseCard({
  course,
  action,
  compact,
  hideDetail,
}: {
  course: CourseCardData;
  action?: ReactNode;
  compact?: boolean;
  hideDetail?: boolean;
}) {
  return (
    <motion.div whileHover={{ y: -6 }} transition={{ duration: 0.2 }}>
      <Card className="group h-full overflow-hidden">
        <div
          className="h-48 bg-cover bg-center"
          style={{
            backgroundImage: `linear-gradient(180deg, rgba(15,23,42,0.04), rgba(15,23,42,0.28)), url(${course.imageUrl || "https://images.unsplash.com/photo-1516321318423-f06f85e504b3?auto=format&fit=crop&w=1200&q=80"})`,
          }}
        />
        <div className="space-y-4 p-5">
          <div className="flex items-center justify-between gap-3">
            <Badge tone="brand">{course.category}</Badge>
            {"status" in course && course.status ? (
              <Badge tone={course.status === "published" ? "success" : "warning"}>
                {course.status}
              </Badge>
            ) : null}
          </div>
          <div>
            <h3 className="font-heading text-xl font-semibold text-slate-950">
              {course.title}
            </h3>
            {!compact ? (
              <p className="mt-2 line-clamp-3 text-sm text-slate-600">
                {"description" in course ? course.description : ""}
              </p>
            ) : null}
          </div>
          <div className="flex items-center justify-between text-sm text-slate-500">
            {"capacity" in course && typeof course.capacity === "number" ? (
              <span className="inline-flex items-center gap-2">
                <Users className="h-4 w-4" />
                {course.capacity} cupos
              </span>
            ) : (
              <span />
            )}
            <span className="font-semibold text-slate-900">
              {formatCurrency(course.price, course.currency)}
            </span>
          </div>
          <div className="flex items-center justify-between gap-3">
            {!hideDetail ? (
              <Link to={`/courses/${course.id}`}>
                <Button variant="ghost" className="px-0 text-brand hover:bg-transparent">
                  Ver detalle
                  <ArrowRight className="h-4 w-4" />
                </Button>
              </Link>
            ) : (
              <span />
            )}
            {action}
          </div>
        </div>
      </Card>
    </motion.div>
  );
}

export function CourseGrid({
  courses,
  action,
  hideDetail,
}: {
  courses: Course[];
  action?: (course: Course) => ReactNode;
  hideDetail?: boolean;
}) {
  return (
    <div className="grid gap-5 sm:grid-cols-2 xl:grid-cols-3">
      {courses.map((course) => (
        <CourseCard
          key={course.id}
          course={course}
          action={action?.(course)}
          hideDetail={hideDetail}
        />
      ))}
    </div>
  );
}

export function LessonSidebar({
  lessons,
  activeLessonId,
  onSelect,
  completedLessonIds = [],
}: {
  lessons: Lesson[];
  activeLessonId?: string;
  onSelect: (lesson: Lesson) => void;
  completedLessonIds?: string[];
}) {
  const completedCount = lessons.filter((lesson) =>
    completedLessonIds.includes(lesson.id),
  ).length;

  return (
    <Card className="overflow-hidden">
      <div className="border-b border-slate-200 px-5 py-4">
        <p className="font-heading text-lg font-semibold text-slate-950">Clases</p>
        <p className="mt-1 text-sm text-slate-500">
          Navega el contenido y retoma donde lo dejaste.
        </p>
        {completedLessonIds.length ? (
          <p className="mt-2 text-xs font-medium uppercase tracking-[0.24em] text-emerald-600">
            {completedCount} de {lessons.length} hechas
          </p>
        ) : null}
      </div>
      <div className="max-h-[70vh] overflow-y-auto p-3">
        <div className="space-y-2">
          {lessons.map((lesson) => {
            const active = lesson.id === activeLessonId;
            const completed = completedLessonIds.includes(lesson.id);

            return (
              <button
                key={lesson.id}
                type="button"
                onClick={() => onSelect(lesson)}
                className={cn(
                  "w-full rounded-2xl p-3 text-left transition",
                  active
                    ? "bg-brand text-white shadow-glow"
                    : "bg-white hover:bg-slate-100",
                )}
              >
                <div className="flex items-center justify-between gap-3">
                  <div>
                    <p className="text-xs font-semibold uppercase tracking-[0.24em] opacity-80">
                      Clase {lesson.orderIndex}
                    </p>
                    <div className="mt-1 flex items-center gap-2">
                      <p className="font-semibold">{lesson.title}</p>
                      {completed ? (
                        <Badge tone={active ? "neutral" : "success"}>Hecha</Badge>
                      ) : null}
                    </div>
                  </div>
                  {completed ? (
                    <CheckCircle2 className="h-5 w-5" />
                  ) : active ? (
                    <PlayCircle className="h-5 w-5" />
                  ) : (
                    <Circle className="h-5 w-5 opacity-60" />
                  )}
                </div>
                <p
                  className={cn(
                    "mt-2 line-clamp-2 text-sm",
                    active ? "text-white/80" : "text-slate-500",
                  )}
                >
                  {lesson.description}
                </p>
              </button>
            );
          })}
        </div>
      </div>
    </Card>
  );
}

export function LessonProgressCard({
  lessons,
  completedLessonIds,
  currentLesson,
  onToggleCurrentLesson,
}: {
  lessons: Lesson[];
  completedLessonIds: string[];
  currentLesson?: Lesson;
  onToggleCurrentLesson: () => void;
}) {
  const completedLessons = lessons.filter((lesson) =>
    completedLessonIds.includes(lesson.id),
  );
  const completedCount = completedLessons.length;
  const total = lessons.length;
  const percent = total ? Math.round((completedCount / total) * 100) : 0;
  const currentLessonCompleted = currentLesson
    ? completedLessonIds.includes(currentLesson.id)
    : false;

  return (
    <Card className="p-6">
      <div className="flex flex-wrap items-start justify-between gap-4">
        <div>
          <p className="text-xs font-semibold uppercase tracking-[0.28em] text-brand">
            Tu progreso
          </p>
          <h3 className="mt-2 font-heading text-2xl font-semibold text-slate-950">
            {completedCount} de {total} clases hechas
          </h3>
          <p className="mt-2 text-sm text-slate-600">
            Marca cada clase cuando la termines para seguir tu avance dentro del
            curso.
          </p>
        </div>
        <Badge tone={completedCount === total && total > 0 ? "success" : "brand"}>
          {percent}%
        </Badge>
      </div>

      <div className="mt-5 h-3 overflow-hidden rounded-full bg-slate-100">
        <div
          className="h-full rounded-full bg-gradient-to-r from-brand to-brand-dark transition-all duration-300"
          style={{ width: `${percent}%` }}
        />
      </div>

      {currentLesson ? (
        <div className="mt-5 rounded-[24px] border border-slate-200 bg-slate-50 p-4">
          <p className="text-xs font-semibold uppercase tracking-[0.24em] text-slate-500">
            Clase actual
          </p>
          <div className="mt-2 flex flex-wrap items-center justify-between gap-3">
            <div>
              <p className="font-semibold text-slate-950">{currentLesson.title}</p>
              <p className="mt-1 text-sm text-slate-600">
                {currentLessonCompleted
                  ? "Ya la marcaste como hecha."
                  : "Cuando la termines, marcala para verla en tu historial."}
              </p>
            </div>
            <Button
              variant={currentLessonCompleted ? "secondary" : "primary"}
              onClick={onToggleCurrentLesson}
            >
              {currentLessonCompleted ? "Quitar marca" : "Marcar como hecha"}
            </Button>
          </div>
        </div>
      ) : null}

      <div className="mt-5">
        <p className="text-xs font-semibold uppercase tracking-[0.24em] text-slate-500">
          Clases completadas
        </p>
        {completedLessons.length ? (
          <div className="mt-3 flex flex-wrap gap-2">
            {completedLessons
              .slice()
              .sort((a, b) => a.orderIndex - b.orderIndex)
              .map((lesson) => (
                <Badge key={lesson.id} tone="success">
                  Clase {lesson.orderIndex}
                </Badge>
              ))}
          </div>
        ) : (
          <p className="mt-2 text-sm text-slate-600">
            Todavia no marcaste ninguna clase como hecha.
          </p>
        )}
      </div>
    </Card>
  );
}

export function VideoPlayer({ lesson }: { lesson: Lesson }) {
  const { token } = useSession();
  const video = getVideoEmbed(lesson.videoUrl);
  const attachments = lesson.attachments ?? [];

  return (
    <Card className="overflow-hidden">
      <div className="border-b border-slate-200 px-6 py-5">
        <div className="flex flex-wrap items-center gap-3">
          <Badge tone="brand">Clase {lesson.orderIndex}</Badge>
          <span className="inline-flex items-center gap-2 text-sm text-slate-500">
            <Clock3 className="h-4 w-4" />
            Disponible al instante
          </span>
        </div>
        <h2 className="mt-3 font-heading text-3xl font-semibold text-slate-950">
          {lesson.title}
        </h2>
        <p className="mt-3 max-w-3xl text-sm leading-6 text-slate-600">
          {lesson.description}
        </p>
      </div>
      <div className="aspect-video w-full bg-slate-950">
        {video?.kind === "iframe" ? (
          <iframe
            className="h-full w-full"
            src={video.src}
            title={lesson.title}
            allow="autoplay; encrypted-media; picture-in-picture"
            allowFullScreen
          />
        ) : video?.kind === "video" ? (
          <video className="h-full w-full" src={video.src} controls />
        ) : (
          <div className="flex h-full flex-col items-center justify-center gap-4 px-6 text-center text-white">
            <PlayCircle className="h-12 w-12 text-brand-soft" />
            <div>
              <p className="font-heading text-2xl font-semibold">
                Abri el recurso externo
              </p>
              <p className="mt-2 text-sm text-white/70">
                Esta leccion apunta a un video externo. Abrilo en una pestana nueva.
              </p>
            </div>
            <a href={lesson.videoUrl} target="_blank" rel="noreferrer">
              <Button>Ir al video</Button>
            </a>
          </div>
        )}
      </div>
      {attachments.length ? (
        <div className="border-t border-slate-200 px-6 py-5">
          <div className="flex items-center justify-between gap-3">
            <div>
              <p className="text-xs font-semibold uppercase tracking-[0.24em] text-brand">
                Material extra
              </p>
              <h3 className="mt-2 font-heading text-2xl font-semibold text-slate-950">
                Fotos, archivos y videos de la clase
              </h3>
            </div>
            <Badge tone="brand">{attachments.length} adjuntos</Badge>
          </div>
          <div className="mt-5 grid gap-4 md:grid-cols-2">
            {attachments.map((attachment) => {
              const assetURL = buildAssetURL(attachment.url, token);
              if (attachment.kind === "image") {
                return (
                  <a
                    key={attachment.id}
                    href={assetURL}
                    target="_blank"
                    rel="noreferrer"
                    className="overflow-hidden rounded-[24px] border border-slate-200 bg-slate-50 transition hover:border-brand/40 hover:bg-white"
                  >
                    <img
                      src={assetURL}
                      alt={attachment.fileName}
                      className="h-56 w-full object-cover"
                    />
                    <div className="flex items-center justify-between gap-3 px-4 py-3">
                      <div>
                        <p className="text-sm font-semibold text-slate-950">
                          {attachment.fileName}
                        </p>
                        <p className="mt-1 text-xs text-slate-500">
                          Imagen descargable
                        </p>
                      </div>
                      <ImageIcon className="h-5 w-5 text-brand" />
                    </div>
                  </a>
                );
              }

              if (attachment.kind === "video") {
                return (
                  <div
                    key={attachment.id}
                    className="overflow-hidden rounded-[24px] border border-slate-200 bg-slate-50"
                  >
                    <video className="h-56 w-full bg-slate-950 object-cover" src={assetURL} controls />
                    <div className="flex items-center justify-between gap-3 px-4 py-3">
                      <div>
                        <p className="text-sm font-semibold text-slate-950">
                          {attachment.fileName}
                        </p>
                        <p className="mt-1 text-xs text-slate-500">
                          Video complementario
                        </p>
                      </div>
                      <Video className="h-5 w-5 text-brand" />
                    </div>
                  </div>
                );
              }

              return (
                <a
                  key={attachment.id}
                  href={assetURL}
                  target="_blank"
                  rel="noreferrer"
                  className="flex items-center justify-between gap-4 rounded-[24px] border border-slate-200 bg-slate-50 px-4 py-4 transition hover:border-brand/40 hover:bg-white"
                >
                  <div className="flex min-w-0 items-center gap-3">
                    <div className="flex h-12 w-12 items-center justify-center rounded-2xl bg-white text-brand shadow-card">
                      <FileText className="h-5 w-5" />
                    </div>
                    <div className="min-w-0">
                      <p className="truncate text-sm font-semibold text-slate-950">
                        {attachment.fileName}
                      </p>
                      <p className="mt-1 text-xs text-slate-500">
                        Archivo adjunto
                      </p>
                    </div>
                  </div>
                  <Download className="h-5 w-5 text-slate-400" />
                </a>
              );
            })}
          </div>
        </div>
      ) : null}
    </Card>
  );
}
