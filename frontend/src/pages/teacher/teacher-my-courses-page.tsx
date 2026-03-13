import { useQuery } from "@tanstack/react-query";
import { ArrowRight, ListChecks, Users } from "lucide-react";
import { Link } from "react-router-dom";

import { coursesApi } from "@/api/endpoints";
import { Button } from "@/components/ui/button";
import { PageIntro } from "@/components/shared/page-intro";
import {
  EmptyState,
  ErrorState,
  LoadingSkeleton,
} from "@/components/shared/feedback";
import { CourseGrid } from "@/features/courses/components";

export function TeacherMyCoursesPage() {
  const query = useQuery({
    queryKey: ["teacher-courses"],
    queryFn: () => coursesApi.teacherCourses(),
  });
  const courses = query.data ?? [];

  return (
    <div className="space-y-8">
      <PageIntro
        eyebrow="Teacher / Cursos"
        title="Cursos donde ya estas asignado."
        description="Desde aca administras los videos del curso, revisas alumnos y entras al aula como vista previa."
      />
      {query.isLoading ? (
        <div className="grid gap-5 sm:grid-cols-2 xl:grid-cols-3">
          {Array.from({ length: 3 }).map((_, index) => (
            <LoadingSkeleton key={index} className="h-[360px]" lines={7} />
          ))}
        </div>
      ) : query.isError ? (
        <ErrorState description={query.error.message} />
      ) : courses.length ? (
        <CourseGrid
          courses={courses}
          hideDetail
          action={(course) => (
            <div className="flex flex-wrap justify-end gap-2">
              <Link to={`/teacher/courses/${course.id}/lessons`}>
                <Button size="sm">
                  <ListChecks className="h-4 w-4" />
                  Gestionar videos
                </Button>
              </Link>
              <Link to={`/teacher/courses/${course.id}/enrollments`}>
                <Button variant="secondary" size="sm">
                  <Users className="h-4 w-4" />
                  Alumnos
                </Button>
              </Link>
              <Link to={`/courses/${course.id}/classroom`}>
                <Button variant="secondary" size="sm">
                  Entrar al aula
                  <ArrowRight className="h-4 w-4" />
                </Button>
              </Link>
            </div>
          )}
        />
      ) : (
        <EmptyState
          title="Todavia no te asignaron cursos"
          description="Cuando un admin te vincule a un curso, lo vas a ver en este panel."
        />
      )}
    </div>
  );
}
