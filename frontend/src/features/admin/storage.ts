import { ADMIN_RECENT_COURSES_KEY } from "@/app/constants";
import type { Course } from "@/api/types";
import { persistJSON, readJSON } from "@/app/utils";

export function getRecentAdminCourses() {
  return readJSON<Course[]>(ADMIN_RECENT_COURSES_KEY, []);
}

export function saveRecentAdminCourse(course: Course) {
  const current = getRecentAdminCourses();
  const next = [course, ...current.filter((item) => item.id !== course.id)].slice(0, 12);
  persistJSON(ADMIN_RECENT_COURSES_KEY, next);
}

export function removeRecentAdminCourse(courseId: string) {
  const current = getRecentAdminCourses();
  persistJSON(
    ADMIN_RECENT_COURSES_KEY,
    current.filter((item) => item.id !== courseId),
  );
}
