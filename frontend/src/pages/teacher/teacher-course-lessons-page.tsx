import { zodResolver } from "@hookform/resolvers/zod";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { ArrowRight, Download, Paperclip, PencilLine, Plus, Trash2, X } from "lucide-react";
import { type Dispatch, type SetStateAction, useState } from "react";
import { useForm } from "react-hook-form";
import { Link, useParams } from "react-router-dom";
import { toast } from "sonner";
import { z } from "zod";

import { buildAssetURL } from "@/api/client";
import { contentApi } from "@/api/endpoints";
import type { Lesson } from "@/api/types";
import { useSession } from "@/auth/session";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Modal } from "@/components/ui/modal";
import { Textarea } from "@/components/ui/textarea";
import { FormField } from "@/components/shared/form-field";
import { PageIntro } from "@/components/shared/page-intro";
import {
  EmptyState,
  ErrorState,
  LoadingSkeleton,
} from "@/components/shared/feedback";

const schema = z.object({
  title: z.string().min(2, "Ingresá un título."),
  description: z.string().min(5, "Ingresá una descripción."),
  orderIndex: z.coerce.number().int().min(1, "El orden arranca en 1."),
  videoUrl: z.string().url("Ingresá una URL válida."),
});

type FormValues = z.infer<typeof schema>;

export function TeacherCourseLessonsPage() {
  const { courseId = "" } = useParams();
  const { token } = useSession();
  const [createOpen, setCreateOpen] = useState(false);
  const [editingLesson, setEditingLesson] = useState<Lesson | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<Lesson | null>(null);
  const [createAttachments, setCreateAttachments] = useState<File[]>([]);
  const [editAttachments, setEditAttachments] = useState<File[]>([]);
  const queryClient = useQueryClient();

  const query = useQuery({
    queryKey: ["teacher-lessons", courseId],
    queryFn: () => contentApi.listLessons(courseId),
    enabled: Boolean(courseId),
  });
  const lessons = query.data ?? [];

  const createForm = useForm<FormValues>({
    resolver: zodResolver(schema),
    defaultValues: {
      title: "",
      description: "",
      orderIndex: 1,
      videoUrl: "",
    },
  });

  const editForm = useForm<FormValues>({
    resolver: zodResolver(schema),
  });

  const createMutation = useMutation({
    mutationFn: (values: FormValues) =>
      contentApi.createLesson(courseId, {
        ...values,
        attachments: createAttachments,
      }),
    onSuccess: () => {
      toast.success("Video agregado al curso.");
      setCreateOpen(false);
      setCreateAttachments([]);
      createForm.reset();
      void queryClient.invalidateQueries({ queryKey: ["teacher-lessons", courseId] });
    },
    onError: (error: Error) => toast.error(error.message),
  });

  const updateMutation = useMutation({
    mutationFn: ({ lessonId, values }: { lessonId: string; values: FormValues }) =>
      contentApi.updateLesson(courseId, lessonId, {
        ...values,
        attachments: editAttachments,
      }),
    onSuccess: () => {
      toast.success("Video actualizado.");
      void queryClient.invalidateQueries({ queryKey: ["teacher-lessons", courseId] });
      setEditAttachments([]);
      setEditingLesson(null);
    },
    onError: (error: Error) => toast.error(error.message),
  });

  const deleteMutation = useMutation({
    mutationFn: (lessonId: string) => contentApi.deleteLesson(courseId, lessonId),
    onSuccess: () => {
      toast.success("Video eliminado del curso.");
      void queryClient.invalidateQueries({ queryKey: ["teacher-lessons", courseId] });
      setDeleteTarget(null);
    },
    onError: (error: Error) => toast.error(error.message),
  });

  function openEdit(lesson: Lesson) {
    setEditingLesson(lesson);
    setEditAttachments([]);
    editForm.reset({
      title: lesson.title,
      description: lesson.description,
      orderIndex: lesson.orderIndex,
      videoUrl: lesson.videoUrl,
    });
  }

  return (
    <div className="space-y-8">
      <PageIntro
        eyebrow="Teacher / Lessons"
        title={`Gestionar videos del curso ${courseId}`}
        description="Como profesor asignado podés cargar, editar y borrar las clases de video de tu curso. No podés crear ni eliminar el curso en sí."
        actions={
          <div className="flex flex-wrap gap-3">
            <Button onClick={() => setCreateOpen(true)}>
              <Plus className="h-4 w-4" />
              Agregar video
            </Button>
            <Link to={`/courses/${courseId}/classroom`}>
              <Button variant="secondary">
                Abrir aula
                <ArrowRight className="h-4 w-4" />
              </Button>
            </Link>
          </div>
        }
      />
      {query.isLoading ? (
        <LoadingSkeleton className="h-72" lines={8} />
      ) : query.isError ? (
        <ErrorState description={query.error.message} />
      ) : lessons.length ? (
        <div className="space-y-4">
          {lessons
            .slice()
            .sort((a, b) => a.orderIndex - b.orderIndex)
            .map((lesson) => {
              const attachments = lesson.attachments ?? [];
              return (
                <Card key={lesson.id} className="p-6">
                  <div className="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
                    <div className="max-w-3xl">
                      <p className="text-xs font-semibold uppercase tracking-[0.24em] text-brand">
                        Clase {lesson.orderIndex}
                      </p>
                      <h2 className="mt-2 font-heading text-2xl font-semibold text-slate-950">
                        {lesson.title}
                      </h2>
                      <p className="mt-3 text-sm leading-6 text-slate-600">
                        {lesson.description}
                      </p>
                      <p className="mt-3 text-xs text-slate-400">{lesson.videoUrl}</p>
                      {attachments.length ? (
                        <div className="mt-4 flex flex-wrap gap-2">
                          {attachments.map((attachment) => (
                            <a
                              key={attachment.id}
                              href={buildAssetURL(attachment.url, token)}
                              target="_blank"
                              rel="noreferrer"
                              className="inline-flex items-center gap-2 rounded-full border border-slate-200 bg-slate-50 px-3 py-1.5 text-xs font-medium text-slate-600 transition hover:border-brand/40 hover:text-brand"
                            >
                              <Paperclip className="h-3.5 w-3.5" />
                              {attachment.fileName}
                            </a>
                          ))}
                        </div>
                      ) : null}
                    </div>
                    <div className="flex gap-2">
                      <Button variant="secondary" size="sm" onClick={() => openEdit(lesson)}>
                        <PencilLine className="h-4 w-4" />
                      </Button>
                      <Button variant="danger" size="sm" onClick={() => setDeleteTarget(lesson)}>
                        <Trash2 className="h-4 w-4" />
                      </Button>
                    </div>
                  </div>
                </Card>
              );
            })}
        </div>
      ) : (
        <EmptyState
          title="Todavía no hay videos cargados"
          description="Podés empezar cargando la primera clase del curso desde este panel."
          action={
            <Button onClick={() => setCreateOpen(true)}>
              <Plus className="h-4 w-4" />
              Cargar primer video
            </Button>
          }
        />
      )}

      <Modal
        open={createOpen}
        title="Agregar video"
        onClose={() => {
          setCreateOpen(false);
          setCreateAttachments([]);
        }}
      >
        <LessonForm
          form={createForm}
          loading={createMutation.isPending}
          submitLabel="Guardar video"
          pendingAttachments={createAttachments}
          setPendingAttachments={setCreateAttachments}
          onSubmit={(values) => createMutation.mutate(values)}
        />
      </Modal>

      <Modal
        open={Boolean(editingLesson)}
        title="Editar video"
        onClose={() => {
          setEditingLesson(null);
          setEditAttachments([]);
        }}
      >
        <LessonForm
          form={editForm}
          loading={updateMutation.isPending}
          submitLabel="Guardar cambios"
          pendingAttachments={editAttachments}
          setPendingAttachments={setEditAttachments}
          existingAttachments={editingLesson?.attachments ?? []}
          token={token}
          onSubmit={(values) => {
            if (!editingLesson) {
              return;
            }
            updateMutation.mutate({ lessonId: editingLesson.id, values });
          }}
        />
      </Modal>

      <Modal
        open={Boolean(deleteTarget)}
        title="Eliminar video"
        description="Esta acción borra la clase del curso."
        onClose={() => setDeleteTarget(null)}
      >
        <div className="space-y-6">
          <p className="text-sm text-slate-600">
            ¿Seguro que querés borrar <strong>{deleteTarget?.title}</strong>?
          </p>
          <div className="flex justify-end gap-3">
            <Button variant="secondary" onClick={() => setDeleteTarget(null)}>
              Cancelar
            </Button>
            <Button
              variant="danger"
              loading={deleteMutation.isPending}
              onClick={() => deleteTarget && deleteMutation.mutate(deleteTarget.id)}
            >
              Eliminar
            </Button>
          </div>
        </div>
      </Modal>
    </div>
  );
}

function LessonForm({
  form,
  onSubmit,
  loading,
  submitLabel,
  pendingAttachments,
  setPendingAttachments,
  existingAttachments = [],
  token,
}: {
  form: ReturnType<typeof useForm<FormValues>>;
  onSubmit: (values: FormValues) => void;
  loading: boolean;
  submitLabel: string;
  pendingAttachments: File[];
  setPendingAttachments: Dispatch<SetStateAction<File[]>>;
  existingAttachments?: Lesson["attachments"];
  token?: string | null;
}) {
  return (
    <form className="space-y-4" onSubmit={form.handleSubmit(onSubmit)}>
      <FormField label="Título" error={form.formState.errors.title?.message}>
        <Input {...form.register("title")} />
      </FormField>
      <FormField
        label="Descripción"
        error={form.formState.errors.description?.message}
      >
        <Textarea {...form.register("description")} />
      </FormField>
      <div className="grid gap-4 sm:grid-cols-[160px_1fr]">
        <FormField
          label="Orden"
          error={form.formState.errors.orderIndex?.message}
        >
          <Input type="number" {...form.register("orderIndex")} />
        </FormField>
        <FormField label="Video URL" error={form.formState.errors.videoUrl?.message}>
          <Input {...form.register("videoUrl")} />
        </FormField>
      </div>
      <div className="space-y-3 rounded-[24px] border border-slate-200 bg-slate-50 p-4">
        <div className="flex items-center justify-between gap-3">
          <div>
            <p className="text-sm font-semibold text-slate-950">Adjuntos opcionales</p>
            <p className="mt-1 text-xs text-slate-500">
              Puedes sumar fotos, archivos y videos extra a esta clase.
            </p>
          </div>
          <label className="inline-flex cursor-pointer items-center gap-2 rounded-full border border-slate-200 bg-white px-3 py-2 text-xs font-semibold text-slate-700 transition hover:border-brand/40 hover:text-brand">
            <Paperclip className="h-4 w-4" />
            Agregar archivos
            <input
              type="file"
              multiple
              className="hidden"
              onChange={(event) => {
                const files = Array.from(event.target.files ?? []);
                if (files.length) {
                  setPendingAttachments((current) => [...current, ...files]);
                }
                event.target.value = "";
              }}
            />
          </label>
        </div>

        {existingAttachments.length ? (
          <div className="space-y-2">
            <p className="text-xs font-semibold uppercase tracking-[0.22em] text-slate-500">
              Adjuntos actuales
            </p>
            <div className="flex flex-wrap gap-2">
              {existingAttachments.map((attachment) => (
                <a
                  key={attachment.id}
                  href={buildAssetURL(attachment.url, token)}
                  target="_blank"
                  rel="noreferrer"
                  className="inline-flex items-center gap-2 rounded-full border border-slate-200 bg-white px-3 py-1.5 text-xs font-medium text-slate-600 transition hover:border-brand/40 hover:text-brand"
                >
                  <Download className="h-3.5 w-3.5" />
                  {attachment.fileName}
                </a>
              ))}
            </div>
          </div>
        ) : null}

        {pendingAttachments.length ? (
          <div className="space-y-2">
            <p className="text-xs font-semibold uppercase tracking-[0.22em] text-slate-500">
              Se van a subir con este guardado
            </p>
            <div className="space-y-2">
              {pendingAttachments.map((file, index) => (
                <div
                  key={`${file.name}-${file.size}-${index}`}
                  className="flex items-center justify-between gap-3 rounded-2xl border border-slate-200 bg-white px-3 py-2 text-sm"
                >
                  <div className="min-w-0">
                    <p className="truncate font-medium text-slate-900">{file.name}</p>
                    <p className="text-xs text-slate-500">
                      {Math.max(1, Math.round(file.size / 1024))} KB
                    </p>
                  </div>
                  <button
                    type="button"
                    onClick={() =>
                      setPendingAttachments((current) =>
                        current.filter((_, currentIndex) => currentIndex !== index),
                      )
                    }
                    className="inline-flex h-8 w-8 items-center justify-center rounded-full text-slate-400 transition hover:bg-slate-100 hover:text-slate-700"
                  >
                    <X className="h-4 w-4" />
                  </button>
                </div>
              ))}
            </div>
          </div>
        ) : null}
      </div>
      <div className="flex justify-end">
        <Button type="submit" loading={loading}>
          {submitLabel}
        </Button>
      </div>
    </form>
  );
}
