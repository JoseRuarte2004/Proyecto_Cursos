import { useQuery } from "@tanstack/react-query";
import { ArrowRight } from "lucide-react";
import { Link } from "react-router-dom";

import { enrollmentsApi } from "@/api/endpoints";
import { useSession } from "@/auth/session";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { PageIntro } from "@/components/shared/page-intro";
import {
  EmptyState,
  ErrorState,
  LoadingSkeleton,
  SectionMotion,
} from "@/components/shared/feedback";
import { CourseCard } from "@/features/courses/components";
import { readCourseProgress } from "@/features/courses/progress";

export function MyCoursesPage() {
  const { user } = useSession();
  const query = useQuery({
    queryKey: ["my-enrollments"],
    queryFn: () => enrollmentsApi.myEnrollments(),
  });
  const enrollments = query.data ?? [];

  return (
    <div className="page-shell space-y-8">
      <SectionMotion>
        <PageIntro
          eyebrow="Alumno"
          title="Tus compras y accesos en un solo lugar."
          description="Ves reservas pendientes y cursos activos, con entrada directa al aula y una referencia rapida de las clases que ya completaste."
        />
      </SectionMotion>

      {query.isLoading ? (
        <div className="grid gap-5 sm:grid-cols-2 xl:grid-cols-3">
          {Array.from({ length: 3 }).map((_, index) => (
            <LoadingSkeleton key={index} className="h-[360px]" lines={7} />
          ))}
        </div>
      ) : query.isError ? (
        <ErrorState description={query.error.message} />
      ) : enrollments.length ? (
        <div className="grid gap-5 sm:grid-cols-2 xl:grid-cols-3">
          {enrollments.map((item) => {
            const progress =
              user && item.status === "active"
                ? readCourseProgress(item.courseId, user.id)
                : null;

            return (
              <CourseCard
                key={`${item.courseId}-${item.status}`}
                course={{
                  id: item.course.id,
                  title: item.course.title,
                  category: item.course.category,
                  imageUrl: item.course.imageUrl,
                  price: item.course.price,
                  currency: item.course.currency,
                  status: item.course.status,
                  description:
                    item.status === "active"
                      ? "Tu acceso ya esta habilitado. Podes entrar directo al aula."
                      : "Tu pago todavia esta en proceso o pendiente de confirmacion.",
                  capacity: 0,
                }}
                action={
                  <div className="flex flex-col items-end gap-2">
                    <div className="flex items-center gap-3">
                      <Badge tone={item.status === "active" ? "success" : "warning"}>
                        {item.status === "active" ? "Activo" : "Pendiente"}
                      </Badge>
                      {item.status === "active" ? (
                        <Link to={`/courses/${item.courseId}/classroom`}>
                          <Button size="sm">
                            Entrar
                            <ArrowRight className="h-4 w-4" />
                          </Button>
                        </Link>
                      ) : null}
                    </div>
                    {item.status === "active" ? (
                      <p className="text-xs font-medium text-slate-500">
                        {progress?.completedLessonIds.length
                          ? `${progress.completedLessonIds.length} clases hechas`
                          : "Todavia no marcaste clases como hechas"}
                      </p>
                    ) : null}
                  </div>
                }
              />
            );
          })}
        </div>
      ) : (
        <EmptyState
          title="Todavia no compraste cursos"
          description="Cuando reserves y completes el checkout, tus cursos apareceran aca con acceso directo al aula."
          action={
            <Link to="/home">
              <Button>Explorar catalogo</Button>
            </Link>
          }
        />
      )}
    </div>
  );
}
