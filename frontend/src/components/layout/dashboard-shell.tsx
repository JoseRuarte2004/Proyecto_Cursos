import type { ReactNode } from "react";
import { ArrowLeft, LogOut } from "lucide-react";
import { Link, NavLink, Outlet } from "react-router-dom";

import { useSession } from "@/auth/session";
import { cn } from "@/app/utils";
import { Button } from "@/components/ui/button";
import { Logo } from "@/components/shared/logo";

type DashboardItem = {
  to: string;
  label: string;
  icon: ReactNode;
};

export function DashboardShell({
  title,
  subtitle,
  items,
}: {
  title: string;
  subtitle: string;
  items: DashboardItem[];
}) {
  const { user, logout } = useSession();
  const logoLink = user?.role === "admin" ? "/admin" : "/teacher/my-courses";
  const showCatalogBack = user?.role !== "admin" && user?.role !== "teacher";

  return (
    <div className="min-h-screen bg-slate-950/[0.02]">
      <div className="page-shell grid gap-6 lg:grid-cols-[280px_minmax(0,1fr)]">
        <aside className="glass-panel surface-pattern rounded-[32px] p-5 shadow-card lg:sticky lg:top-6 lg:h-[calc(100vh-3rem)]">
          <Link to={logoLink}>
            <Logo />
          </Link>
          <div className="mt-8">
            <p className="font-heading text-2xl font-semibold text-slate-950">
              {title}
            </p>
            <p className="mt-2 text-sm text-slate-600">{subtitle}</p>
          </div>
          <nav className="mt-8 space-y-2">
            {items.map((item) => (
              <NavLink
                key={item.to}
                to={item.to}
                end={item.to === "/admin" || item.to === "/teacher"}
                className={({ isActive }) =>
                  cn(
                    "flex items-center gap-3 rounded-2xl px-4 py-3 text-sm font-medium transition",
                    isActive
                      ? "bg-brand text-white shadow-glow"
                      : "text-slate-700 hover:bg-white hover:shadow-card",
                  )
                }
              >
                {item.icon}
                {item.label}
              </NavLink>
            ))}
          </nav>
          <div className="mt-8 space-y-3">
            {showCatalogBack ? (
              <Link to="/home">
                <Button variant="secondary" className="w-full justify-center">
                  <ArrowLeft className="h-4 w-4" />
                  Volver al catalogo
                </Button>
              </Link>
            ) : null}
            <Button variant="secondary" className="w-full justify-center" onClick={logout}>
              <LogOut className="h-4 w-4" />
              Cerrar sesion
            </Button>
          </div>
        </aside>
        <div className="min-w-0">
          <Outlet />
        </div>
      </div>
    </div>
  );
}
