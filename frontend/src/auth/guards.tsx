import { Navigate, Outlet, useLocation } from "react-router-dom";

import { LoaderScreen } from "@/components/shared/feedback";
import { useSession } from "@/auth/session";
import type { Role } from "@/api/types";

export function RequireAuth() {
  const { token, isBootstrapping } = useSession();
  const location = useLocation();

  if (isBootstrapping) {
    return <LoaderScreen label="Verificando tu sesión..." />;
  }

  if (!token) {
    return <Navigate to="/login" replace state={{ from: location.pathname }} />;
  }

  return <Outlet />;
}

export function RedirectAdminToDashboard() {
  const { user, isBootstrapping } = useSession();
  const location = useLocation();

  if (isBootstrapping) {
    return <LoaderScreen label="Cargando permisos..." />;
  }

  if (user?.role === "admin") {
    return <Navigate to="/admin" replace />;
  }
  if (user?.role === "teacher") {
    if (isTeacherAllowedPublicPath(location.pathname)) {
      return <Outlet />;
    }
    return <Navigate to="/teacher/my-courses" replace />;
  }

  return <Outlet />;
}

export function RequireRole({ role }: { role: Role }) {
  const { user, isBootstrapping } = useSession();

  if (isBootstrapping) {
    return <LoaderScreen label="Cargando permisos..." />;
  }

  if (!user || user.role !== role) {
    if (user?.role === "admin") {
      return <Navigate to="/admin" replace />;
    }
    if (user?.role === "teacher") {
      return <Navigate to="/teacher" replace />;
    }
    return <Navigate to="/home" replace />;
  }

  return <Outlet />;
}

function isTeacherAllowedPublicPath(pathname: string) {
  const segments = pathname.split("/").filter(Boolean);
  return segments.length === 3 && segments[0] === "courses" && segments[2] === "classroom";
}
