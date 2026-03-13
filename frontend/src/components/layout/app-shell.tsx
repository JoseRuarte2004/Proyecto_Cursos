import { AnimatePresence, motion } from "framer-motion";
import { BookOpen, Home, LogOut, Shield, UserCircle2 } from "lucide-react";
import { Link, NavLink, Outlet, useLocation } from "react-router-dom";

import { Button } from "@/components/ui/button";
import { Logo } from "@/components/shared/logo";
import { Badge } from "@/components/ui/badge";
import { useSession } from "@/auth/session";

export function AppShell() {
  const { user, logout } = useSession();
  const location = useLocation();
  const homeLink = user?.role === "admin" ? "/admin" : "/home";

  return (
    <div className="min-h-screen">
      <header className="sticky top-0 z-40 border-b border-white/70 bg-white/80 backdrop-blur-xl">
        <div className="page-shell flex flex-col gap-4 py-4 sm:flex-row sm:items-center sm:justify-between">
          <div className="flex items-center justify-between gap-4">
            <Link to={homeLink}>
              <Logo />
            </Link>
            <nav className="hidden items-center gap-2 rounded-full border border-slate-200 bg-white/90 px-2 py-1 md:flex">
              {user?.role !== "admin" ? (
                <ShellLink to="/home" icon={Home} label="Inicio" />
              ) : null}
              {user?.role === "student" ? (
                <ShellLink to="/me/courses" icon={BookOpen} label="Mis cursos" />
              ) : null}
              {user?.role === "admin" ? (
                <ShellLink to="/admin" icon={Shield} label="Admin" />
              ) : null}
              {user?.role === "teacher" ? (
                <ShellLink to="/teacher" icon={BookOpen} label="Teacher" />
              ) : null}
            </nav>
          </div>
          <div className="flex items-center gap-3">
            {user ? (
              <>
                <Badge tone="brand">{user.role}</Badge>
                <div className="hidden text-right sm:block">
                  <p className="text-sm font-semibold text-slate-900">{user.name}</p>
                  <p className="text-xs text-slate-500">{user.email}</p>
                </div>
                <Button variant="secondary" size="sm" onClick={logout}>
                  <LogOut className="h-4 w-4" />
                  Salir
                </Button>
              </>
            ) : (
              <div className="flex gap-2">
                <Link to="/login">
                  <Button variant="secondary" size="sm">
                    Ingresar
                  </Button>
                </Link>
                <Link to="/register">
                  <Button size="sm">Crear cuenta</Button>
                </Link>
              </div>
            )}
          </div>
        </div>
      </header>

      <AnimatePresence mode="wait">
        <motion.main
          key={location.pathname}
          initial={{ opacity: 0, y: 8 }}
          animate={{ opacity: 1, y: 0 }}
          exit={{ opacity: 0, y: -6 }}
          transition={{ duration: 0.25 }}
        >
          <Outlet />
        </motion.main>
      </AnimatePresence>
    </div>
  );
}

function ShellLink({
  to,
  label,
  icon: Icon,
}: {
  to: string;
  label: string;
  icon: typeof UserCircle2;
}) {
  return (
    <NavLink
      to={to}
      className={({ isActive }) =>
        [
          "flex items-center gap-2 rounded-full px-4 py-2 text-sm font-medium transition",
          isActive
            ? "bg-brand text-white shadow-glow"
            : "text-slate-600 hover:bg-slate-100 hover:text-slate-950",
        ].join(" ")
      }
    >
      <Icon className="h-4 w-4" />
      {label}
    </NavLink>
  );
}
