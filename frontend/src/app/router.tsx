import {
  BookCopy,
  BookOpen,
  LayoutDashboard,
  Users,
} from "lucide-react";
import { Navigate, RouterProvider, createBrowserRouter } from "react-router-dom";

import { RedirectAdminToDashboard, RequireAuth, RequireRole } from "@/auth/guards";
import { useSession } from "@/auth/session";
import { AppShell } from "@/components/layout/app-shell";
import { AuthLayout } from "@/components/layout/auth-layout";
import { DashboardShell } from "@/components/layout/dashboard-shell";
import { LoaderScreen } from "@/components/shared/feedback";
import { AdminDashboardPage } from "@/pages/admin/admin-dashboard-page";
import { AdminCoursesPage } from "@/pages/admin/admin-courses-page";
import { AdminCourseLessonsPage } from "@/pages/admin/admin-course-lessons-page";
import { AdminCourseTeachersPage } from "@/pages/admin/admin-course-teachers-page";
import { AdminEnrollmentsPage } from "@/pages/admin/admin-enrollments-page";
import { AdminUsersPage } from "@/pages/admin/admin-users-page";
import { CheckoutFailurePage } from "@/pages/checkout-failure-page";
import { CheckoutRedirectPage } from "@/pages/checkout-redirect-page";
import { CheckoutSuccessPage } from "@/pages/checkout-success-page";
import { ClassroomPage } from "@/pages/classroom-page";
import { CourseDetailPage } from "@/pages/course-detail-page";
import { ForgotPasswordPage } from "@/pages/forgot-password-page";
import { HomePage } from "@/pages/home-page";
import { LoginPage } from "@/pages/login-page";
import { MyCoursesPage } from "@/pages/my-courses-page";
import { NotFoundPage } from "@/pages/not-found-page";
import { RegisterPage } from "@/pages/register-page";
import { TeacherCourseEnrollmentsPage } from "@/pages/teacher/teacher-course-enrollments-page";
import { TeacherCourseLessonsPage } from "@/pages/teacher/teacher-course-lessons-page";
import { TeacherMyCoursesPage } from "@/pages/teacher/teacher-my-courses-page";

function RootRedirect() {
  const { user, isBootstrapping } = useSession();

  if (isBootstrapping) {
    return <LoaderScreen label="Cargando tu espacio..." />;
  }

  if (user?.role === "admin") {
    return <Navigate to="/admin" replace />;
  }
  if (user?.role === "teacher") {
    return <Navigate to="/teacher" replace />;
  }
  return <Navigate to="/home" replace />;
}

const router = createBrowserRouter([
  {
    path: "/",
    element: <RootRedirect />,
  },
  {
    element: <AuthLayout />,
    children: [
      {
        path: "/login",
        element: <LoginPage />,
      },
      {
        path: "/register",
        element: <RegisterPage />,
      },
      {
        path: "/forgot-password",
        element: <ForgotPasswordPage />,
      },
    ],
  },
  {
    element: <RedirectAdminToDashboard />,
    children: [
      {
        element: <AppShell />,
        children: [
          {
            path: "/home",
            element: <HomePage />,
          },
          {
            path: "/courses/:id",
            element: <CourseDetailPage />,
          },
          {
            path: "/checkout/redirect",
            element: <CheckoutRedirectPage />,
          },
          {
            path: "/checkout/success",
            element: <CheckoutSuccessPage />,
          },
          {
            path: "/checkout/failure",
            element: <CheckoutFailurePage />,
          },
          {
            element: <RequireAuth />,
            children: [
              {
                path: "/me/courses",
                element: <MyCoursesPage />,
              },
              {
                path: "/courses/:courseId/classroom",
                element: <ClassroomPage />,
              },
            ],
          },
        ],
      },
    ],
  },
  {
    element: <RequireAuth />,
    children: [
      {
        element: <RequireRole role="admin" />,
        children: [
          {
            path: "/admin",
            element: (
              <DashboardShell
                title="Admin"
                subtitle="Catálogo, usuarios, docentes y contenido."
                items={[
                  {
                    to: "/admin",
                    label: "Dashboard",
                    icon: <LayoutDashboard className="h-4 w-4" />,
                  },
                  {
                    to: "/admin/courses",
                    label: "Cursos",
                    icon: <BookCopy className="h-4 w-4" />,
                  },
                  {
                    to: "/admin/users",
                    label: "Usuarios",
                    icon: <Users className="h-4 w-4" />,
                  },
                  {
                    to: "/admin/enrollments",
                    label: "Inscripciones",
                    icon: <BookOpen className="h-4 w-4" />,
                  },
                ]}
              />
            ),
            children: [
              {
                index: true,
                element: <AdminDashboardPage />,
              },
              {
                path: "courses",
                element: <AdminCoursesPage />,
              },
              {
                path: "courses/:id/teachers",
                element: <AdminCourseTeachersPage />,
              },
              {
                path: "courses/:courseId/lessons",
                element: <AdminCourseLessonsPage />,
              },
              {
                path: "users",
                element: <AdminUsersPage />,
              },
              {
                path: "enrollments",
                element: <AdminEnrollmentsPage />,
              },
            ],
          },
        ],
      },
      {
        element: <RequireRole role="teacher" />,
        children: [
          {
            path: "/teacher",
            element: (
              <DashboardShell
                title="Teacher"
                subtitle="Cursos asignados, alumnos y lectura del contenido."
                items={[
                  {
                    to: "/teacher/my-courses",
                    label: "Mis cursos",
                    icon: <BookCopy className="h-4 w-4" />,
                  },
                ]}
              />
            ),
            children: [
              {
                index: true,
                element: <Navigate to="my-courses" replace />,
              },
              {
                path: "my-courses",
                element: <TeacherMyCoursesPage />,
              },
              {
                path: "courses/:courseId/enrollments",
                element: <TeacherCourseEnrollmentsPage />,
              },
              {
                path: "courses/:courseId/lessons",
                element: <TeacherCourseLessonsPage />,
              },
            ],
          },
        ],
      },
    ],
  },
  {
    path: "*",
    element: <NotFoundPage />,
  },
]);

export function AppRouter() {
  return <RouterProvider router={router} />;
}
