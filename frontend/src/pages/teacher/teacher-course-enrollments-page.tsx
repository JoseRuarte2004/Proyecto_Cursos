import { useQuery } from "@tanstack/react-query";
import { useParams } from "react-router-dom";

import { enrollmentsApi } from "@/api/endpoints";
import { PageIntro } from "@/components/shared/page-intro";
import {
  EmptyState,
  ErrorState,
  LoadingSkeleton,
} from "@/components/shared/feedback";
import { Badge } from "@/components/ui/badge";
import { DataTable } from "@/components/ui/table";

export function TeacherCourseEnrollmentsPage() {
  const { courseId = "" } = useParams();
  const query = useQuery({
    queryKey: ["teacher-course-enrollments", courseId],
    queryFn: () => enrollmentsApi.teacherCourseEnrollments(courseId),
    enabled: Boolean(courseId),
  });

  const enrollments = query.data?.enrollments ?? [];
  const courseTitle = query.data?.courseTitle?.trim() || "este curso";

  return (
    <div className="space-y-8">
      <PageIntro
        eyebrow="Teacher / Alumnos"
        title={`Alumnos registrados en ${courseTitle}`}
        description="Aca podes ver los alumnos registrados en el curso y el estado actual de cada inscripcion."
      />
      {query.isLoading ? (
        <LoadingSkeleton className="h-72" lines={8} />
      ) : query.isError ? (
        <ErrorState description={query.error.message} />
      ) : enrollments.length ? (
        <DataTable
          columns={["Alumno", "Estado", "Registrado"]}
          rows={enrollments.map((item) => [
            item.studentName,
            <Badge
              key={`${item.studentName}-${item.createdAt}-status`}
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
          title="Todavia no hay alumnos registrados"
          description="Cuando se registren alumnos en el curso, los vas a ver reflejados aca."
        />
      )}
    </div>
  );
}
