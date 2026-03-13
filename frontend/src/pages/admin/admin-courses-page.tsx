import { zodResolver } from "@hookform/resolvers/zod";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { PencilLine, Plus, Trash2, Users2, Video } from "lucide-react";
import { useMemo, useState } from "react";
import { useForm } from "react-hook-form";
import { Link } from "react-router-dom";
import { toast } from "sonner";
import { z } from "zod";

import { coursesApi } from "@/api/endpoints";
import type { Course } from "@/api/types";
import {
  getRecentAdminCourses,
  removeRecentAdminCourse,
  saveRecentAdminCourse,
} from "@/features/admin/storage";
import { CourseCard } from "@/features/courses/components";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Modal } from "@/components/ui/modal";
import { Select } from "@/components/ui/select";
import { Textarea } from "@/components/ui/textarea";
import { PageIntro } from "@/components/shared/page-intro";
import { FormField } from "@/components/shared/form-field";
import {
  EmptyState,
  ErrorState,
  LoadingSkeleton,
} from "@/components/shared/feedback";

const schema = z.object({
  title: z.string().min(3, "Ingresá un título."),
  description: z.string().min(10, "La descripción debe ser más clara."),
  category: z.string().min(2, "Ingresá una categoría."),
  imageUrl: z.string().url("Ingresá una URL válida.").or(z.literal("")),
  price: z.coerce.number().min(0, "El precio debe ser positivo."),
  currency: z.string().min(3, "Ingresá la moneda."),
  capacity: z.coerce.number().int().min(1, "La capacidad debe ser mayor a cero."),
  status: z.enum(["draft", "published"]),
});

type FormValues = z.infer<typeof schema>;

export function AdminCoursesPage() {
  const [editingCourse, setEditingCourse] = useState<Course | null>(null);
  const [createOpen, setCreateOpen] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<Course | null>(null);
  const [recentCourses, setRecentCourses] = useState<Course[]>(() =>
    getRecentAdminCourses(),
  );
  const queryClient = useQueryClient();

  const publishedQuery = useQuery({
    queryKey: ["admin-published-courses"],
    queryFn: () => coursesApi.listPublished(50, 0),
  });

  const courses = useMemo(() => {
    const published = publishedQuery.data ?? [];
    const merged = [...recentCourses];
    for (const course of published) {
      if (!merged.find((item) => item.id === course.id)) {
        merged.push(course);
      }
    }
    return merged;
  }, [publishedQuery.data, recentCourses]);

  const createForm = useForm<FormValues>({
    resolver: zodResolver(schema),
    defaultValues: {
      title: "",
      description: "",
      category: "backend",
      imageUrl: "",
      price: 0,
      currency: "ARS",
      capacity: 20,
      status: "draft",
    },
  });

  const editForm = useForm<FormValues>({
    resolver: zodResolver(schema),
  });

  const createMutation = useMutation({
    mutationFn: (values: FormValues) =>
      coursesApi.create({
        ...values,
        imageUrl: values.imageUrl || null,
      }),
    onSuccess: (course) => {
      toast.success("Curso creado.");
      saveRecentAdminCourse(course);
      setRecentCourses((current) => [
        course,
        ...current.filter((item) => item.id !== course.id),
      ]);
      void queryClient.invalidateQueries({ queryKey: ["admin-published-courses"] });
      setCreateOpen(false);
      createForm.reset();
    },
    onError: (error: Error) => toast.error(error.message),
  });

  const updateMutation = useMutation({
    mutationFn: ({ id, values }: { id: string; values: FormValues }) =>
      coursesApi.update(id, {
        ...values,
        imageUrl: values.imageUrl || null,
      }),
    onSuccess: (course) => {
      toast.success("Curso actualizado.");
      saveRecentAdminCourse(course);
      setRecentCourses((current) => [
        course,
        ...current.filter((item) => item.id !== course.id),
      ]);
      void queryClient.invalidateQueries({ queryKey: ["admin-published-courses"] });
      setEditingCourse(course);
    },
    onError: (error: Error) => toast.error(error.message),
  });

  const deleteMutation = useMutation({
    mutationFn: (courseId: string) => coursesApi.delete(courseId),
    onSuccess: () => {
      if (deleteTarget) {
        removeRecentAdminCourse(deleteTarget.id);
        setRecentCourses((current) =>
          current.filter((item) => item.id !== deleteTarget.id),
        );
      }
      toast.success("Curso eliminado.");
      void queryClient.invalidateQueries({ queryKey: ["admin-published-courses"] });
      setDeleteTarget(null);
    },
    onError: (error: Error) => toast.error(error.message),
  });

  function openEdit(course: Course) {
    setEditingCourse(course);
    editForm.reset({
      title: course.title,
      description: course.description,
      category: course.category,
      imageUrl: course.imageUrl ?? "",
      price: course.price,
      currency: course.currency,
      capacity: course.capacity,
      status: course.status,
    });
  }

  return (
    <div className="space-y-8">
      <PageIntro
        eyebrow="Admin / Cursos"
        title="Crear, publicar y mantener cursos sin romper el catálogo."
        description="El panel mezcla el catálogo publicado con los cursos recientes que administraste desde este navegador, para cubrir también drafts creados o editados acá."
        actions={
          <Button onClick={() => setCreateOpen(true)}>
            <Plus className="h-4 w-4" />
            Nuevo curso
          </Button>
        }
      />

      {publishedQuery.isLoading ? (
        <div className="grid gap-5 sm:grid-cols-2 xl:grid-cols-3">
          {Array.from({ length: 3 }).map((_, index) => (
            <LoadingSkeleton key={index} className="h-[390px]" lines={8} />
          ))}
        </div>
      ) : publishedQuery.isError ? (
        <ErrorState description={publishedQuery.error.message} />
      ) : courses.length ? (
        <div className="grid gap-5 sm:grid-cols-2 xl:grid-cols-3">
          {courses.map((course) => (
            <CourseCard
              key={course.id}
              compact
              course={course}
              action={
                <div className="flex flex-wrap justify-end gap-2">
                  <Link to={`/admin/courses/${course.id}/teachers`}>
                    <Button variant="secondary" size="sm">
                      <Users2 className="h-4 w-4" />
                    </Button>
                  </Link>
                  <Link to={`/admin/courses/${course.id}/lessons`}>
                    <Button variant="secondary" size="sm">
                      <Video className="h-4 w-4" />
                    </Button>
                  </Link>
                  <Button variant="secondary" size="sm" onClick={() => openEdit(course)}>
                    <PencilLine className="h-4 w-4" />
                  </Button>
                  <Button variant="danger" size="sm" onClick={() => setDeleteTarget(course)}>
                    <Trash2 className="h-4 w-4" />
                  </Button>
                </div>
              }
            />
          ))}
        </div>
      ) : (
        <EmptyState
          title="Todavía no hay cursos administrados"
          description="Creá el primero desde acá. Los drafts que generes quedan visibles como recientes en este navegador."
          action={
            <Button onClick={() => setCreateOpen(true)}>
              <Plus className="h-4 w-4" />
              Crear curso
            </Button>
          }
        />
      )}

      <Modal
        open={createOpen}
        title="Crear curso"
        onClose={() => setCreateOpen(false)}
      >
        <CourseForm
          form={createForm}
          onSubmit={(values) => createMutation.mutate(values)}
          loading={createMutation.isPending}
          submitLabel="Crear curso"
        />
      </Modal>

      <Modal
        open={Boolean(editingCourse)}
        title="Editar curso"
        description={
          editingCourse
            ? `Estás editando ${editingCourse.title}.`
            : undefined
        }
        onClose={() => setEditingCourse(null)}
      >
        <CourseForm
          form={editForm}
          onSubmit={(values) => {
            if (!editingCourse) {
              return;
            }
            updateMutation.mutate({ id: editingCourse.id, values });
          }}
          loading={updateMutation.isPending}
          submitLabel="Guardar cambios"
        />
      </Modal>

      <Modal
        open={Boolean(deleteTarget)}
        title="Eliminar curso"
        description="Esta acción elimina el curso y sus asignaciones."
        onClose={() => setDeleteTarget(null)}
      >
        <div className="space-y-6">
          <p className="text-sm text-slate-600">
            ¿Seguro que querés eliminar <strong>{deleteTarget?.title}</strong>?
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

function CourseForm({
  form,
  onSubmit,
  loading,
  submitLabel,
}: {
  form: ReturnType<typeof useForm<FormValues>>;
  onSubmit: (values: FormValues) => void;
  loading: boolean;
  submitLabel: string;
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
      <div className="grid gap-4 sm:grid-cols-2">
        <FormField label="Categoría" error={form.formState.errors.category?.message}>
          <Input {...form.register("category")} />
        </FormField>
        <FormField label="Imagen URL" error={form.formState.errors.imageUrl?.message}>
          <Input {...form.register("imageUrl")} />
        </FormField>
      </div>
      <div className="grid gap-4 sm:grid-cols-3">
        <FormField label="Precio" error={form.formState.errors.price?.message}>
          <Input type="number" step="0.01" {...form.register("price")} />
        </FormField>
        <FormField
          label="Moneda"
          error={form.formState.errors.currency?.message}
        >
          <Input {...form.register("currency")} />
        </FormField>
        <FormField
          label="Capacidad"
          error={form.formState.errors.capacity?.message}
        >
          <Input type="number" {...form.register("capacity")} />
        </FormField>
      </div>
      <FormField label="Estado" error={form.formState.errors.status?.message}>
        <Select {...form.register("status")}>
          <option value="draft">draft</option>
          <option value="published">published</option>
        </Select>
      </FormField>
      <div className="flex justify-end">
        <Button type="submit" loading={loading}>
          {submitLabel}
        </Button>
      </div>
    </form>
  );
}
