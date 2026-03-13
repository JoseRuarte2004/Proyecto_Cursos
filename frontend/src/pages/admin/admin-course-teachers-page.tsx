import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Search, Trash2, UserPlus } from "lucide-react";
import { useEffect, useMemo, useState } from "react";
import { Link, useParams } from "react-router-dom";
import { toast } from "sonner";

import { coursesApi, usersApi } from "@/api/endpoints";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { PageIntro } from "@/components/shared/page-intro";
import {
  EmptyState,
  ErrorState,
  LoadingSkeleton,
} from "@/components/shared/feedback";
import { removeRecentAdminCourse } from "@/features/admin/storage";

export function AdminCourseTeachersPage() {
  const { id = "" } = useParams();
  const [teacherId, setTeacherId] = useState("");
  const [search, setSearch] = useState("");
  const queryClient = useQueryClient();

  const teachersQuery = useQuery({
    queryKey: ["course-teachers", id],
    queryFn: () => coursesApi.listTeachers(id),
    enabled: Boolean(id),
  });
  const teacherIds = teachersQuery.data?.teacherIds ?? [];
  const courseWasRemoved =
    teachersQuery.isError && teachersQuery.error.message === "course not found";

  const usersQuery = useQuery({
    queryKey: ["admin-users-lookup"],
    queryFn: () => usersApi.listAdminUsers(),
  });

  useEffect(() => {
    if (!courseWasRemoved || !id) {
      return;
    }

    removeRecentAdminCourse(id);
  }, [courseWasRemoved, id]);

  const candidates = useMemo(() => {
    return (usersQuery.data ?? [])
      .filter((user) => user.role === "teacher")
      .filter((user) => !teacherIds.includes(user.id))
      .filter((user) =>
        !search
          ? true
          : [user.name, user.email, user.id]
              .join(" ")
              .toLowerCase()
              .includes(search.toLowerCase()),
      );
  }, [search, teachersQuery.data?.teacherIds, usersQuery.data]);

  const assignMutation = useMutation({
    mutationFn: (nextTeacherId: string) => coursesApi.assignTeacher(id, nextTeacherId),
    onSuccess: () => {
      toast.success("Profesor asignado.");
      setTeacherId("");
      void queryClient.invalidateQueries({ queryKey: ["course-teachers", id] });
    },
    onError: (error: Error) => toast.error(error.message),
  });

  const removeMutation = useMutation({
    mutationFn: (nextTeacherId: string) => coursesApi.removeTeacher(id, nextTeacherId),
    onSuccess: () => {
      toast.success("Profesor desasignado.");
      void queryClient.invalidateQueries({ queryKey: ["course-teachers", id] });
    },
    onError: (error: Error) => toast.error(error.message),
  });

  return (
    <div className="space-y-8">
      <PageIntro
        eyebrow="Admin / Profesores"
        title={`Asignaciones del curso ${id}`}
        description="Podés pegar un ID manualmente o buscar docentes existentes y asignarlos sin salir del panel."
      />

      <Card className="p-6">
        <div className="grid gap-4 lg:grid-cols-[1fr_auto]">
          <Input
            placeholder="Pegá un teacherId"
            value={teacherId}
            onChange={(event) => setTeacherId(event.target.value)}
          />
          <Button
            loading={assignMutation.isPending}
            onClick={() => assignMutation.mutate(teacherId)}
          >
            <UserPlus className="h-4 w-4" />
            Asignar
          </Button>
        </div>
      </Card>

      {teachersQuery.isLoading ? (
        <LoadingSkeleton className="h-56" lines={6} />
      ) : teachersQuery.isError ? (
        <ErrorState
          title={courseWasRemoved ? "Curso no encontrado" : undefined}
          description={
            courseWasRemoved
              ? "Ese curso ya no existe. Lo quité de tus recientes para que no vuelva a aparecer."
              : teachersQuery.error.message
          }
          action={
            courseWasRemoved ? (
              <Link to="/admin/courses">
                <Button variant="secondary">Volver a cursos</Button>
              </Link>
            ) : undefined
          }
        />
      ) : (
        <div className="grid gap-5 lg:grid-cols-2">
          <Card className="p-6">
            <h2 className="font-heading text-2xl font-semibold text-slate-950">
              Profesores asignados
            </h2>
            <div className="mt-5 space-y-3">
              {teacherIds.length ? (
                teacherIds.map((item) => (
                  <div
                    key={item}
                    className="flex items-center justify-between gap-3 rounded-2xl border border-slate-200 p-4"
                  >
                    <div>
                      <p className="font-semibold text-slate-950">{item}</p>
                      <p className="text-sm text-slate-500">
                        {(usersQuery.data ?? []).find((user) => user.id === item)?.email ??
                          "Docente cargado por ID"}
                      </p>
                    </div>
                    <Button
                      variant="danger"
                      size="sm"
                      onClick={() => removeMutation.mutate(item)}
                    >
                      <Trash2 className="h-4 w-4" />
                    </Button>
                  </div>
                ))
              ) : (
                <EmptyState
                  title="Sin profesores asignados"
                  description="Todavía no hay docentes vinculados a este curso."
                />
              )}
            </div>
          </Card>

          <Card className="p-6">
            <div className="flex items-center gap-3">
              <Search className="h-4 w-4 text-slate-400" />
              <Input
                placeholder="Filtrar docentes por nombre, email o ID"
                value={search}
                onChange={(event) => setSearch(event.target.value)}
              />
            </div>
            <div className="mt-5 space-y-3">
              {candidates.length ? (
                candidates.map((candidate) => (
                  <div
                    key={candidate.id}
                    className="flex items-center justify-between gap-3 rounded-2xl border border-slate-200 p-4"
                  >
                    <div>
                      <p className="font-semibold text-slate-950">{candidate.name}</p>
                      <p className="text-sm text-slate-500">{candidate.email}</p>
                      <p className="text-xs text-slate-400">{candidate.id}</p>
                    </div>
                    <Button
                      size="sm"
                      onClick={() => assignMutation.mutate(candidate.id)}
                    >
                      Asignar
                    </Button>
                  </div>
                ))
              ) : (
                <EmptyState
                  title="Sin candidatos"
                  description="No encontramos más docentes disponibles con ese filtro."
                />
              )}
            </div>
          </Card>
        </div>
      )}
    </div>
  );
}
