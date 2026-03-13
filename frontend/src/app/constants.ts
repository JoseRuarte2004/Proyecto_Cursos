export const SESSION_STORAGE_KEY = "cursos_online_session";
export const ADMIN_RECENT_COURSES_KEY = "cursos_online_admin_recent_courses";
export const LAST_LESSON_KEY_PREFIX = "cursos_online_last_lesson:";
export const LESSON_PROGRESS_KEY_PREFIX = "cursos_online_lesson_progress:";

function resolveDefaultApiBaseURL() {
  if (typeof window === "undefined") {
    return "http://localhost:8080/api";
  }

  const { origin, port } = window.location;
  if (port === "5173") {
    return "http://localhost:8080/api";
  }

  return `${origin}/api`;
}

export const API_BASE_URL =
  import.meta.env.VITE_API_BASE_URL ?? resolveDefaultApiBaseURL();
