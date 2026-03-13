import { useQuery } from "@tanstack/react-query";

import { enrollmentsApi } from "@/api/endpoints";
import { PageIntro } from "@/components/shared/page-intro";
import {
  EmptyState,
  ErrorState,
  LoadingSkeleton,
} from "@/components/shared/feedback";
import { Badge } from "@/components/ui/badge";
import { DataTable } from "@/components/ui/table";

export function AdminEnrollmentsPage() {
  const query = useQuery({
    queryKey: ["admin-enrollments"],
    queryFn: () => enrollmentsApi.adminEnrollments(),
  });
  const enrollments = query.data ?? [];

  return (
    <div className="space-y-8">
      <PageIntro
        eyebrow="Admin / Inscripciones"
        title="Inscripciones de la plataforma"
        description="Aca podes revisar los alumnos inscriptos, el curso correspondiente y el estado actual de cada registro."
      />
      {query.isLoading ? (
        <LoadingSkeleton className="h-72" lines={8} />
      ) : query.isError ? (
        <ErrorState description={query.error.message} />
      ) : enrollments.length ? (
        <DataTable
          columns={["Alumno", "Curso", "Estado", "Registrado"]}
          rows={enrollments.map((item) => [
            item.studentName,
            item.courseTitle,
            <Badge
              key={`${item.studentName}-${item.courseTitle}-${item.createdAt}-status`}
              tone={item.status === "active" ? "success" : "warning"}
            >
              {item.status}
            </Badge>,
            new Date(item.createdAt).toLocaleString("es-AR", {
              dateStyle: "short",
              timeStyle: "short",
            }),
          ])}
        />
      ) : (
        <EmptyState
          title="Todavia no hay inscripciones"
          description="Cuando un alumno reserve un cupo o se active un pago, lo vas a ver reflejado aca."
        />
      )}
    </div>
  );
}
