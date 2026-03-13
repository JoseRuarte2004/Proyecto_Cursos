import { BookCopy, BookOpenCheck, ShieldCheck, Users } from "lucide-react";
import { Link } from "react-router-dom";

import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { PageIntro } from "@/components/shared/page-intro";

const items = [
  {
    to: "/admin/courses",
    icon: BookCopy,
    title: "Gestionar cursos",
    description: "Crear, editar, publicar y eliminar cursos del catálogo.",
  },
  {
    to: "/admin/users",
    icon: Users,
    title: "Gestionar usuarios",
    description: "Ver datos sensibles y cambiar roles entre alumno, profe y admin.",
  },
  {
    to: "/admin/enrollments",
    icon: BookOpenCheck,
    title: "Ver inscripciones",
    description: "Seguir reservas y accesos activos desde una sola tabla.",
  },
];

export function AdminDashboardPage() {
  return (
    <div className="space-y-8">
      <PageIntro
        eyebrow="Admin"
        title="Centro de control del catálogo y los accesos."
        description="Usá estos accesos para crear cursos, asignar profesores, cargar clases y administrar usuarios sin salir del panel."
      />
      <div className="grid gap-5 lg:grid-cols-3">
        {items.map((item) => (
          <Card key={item.to} className="p-6">
            <div className="flex h-12 w-12 items-center justify-center rounded-2xl bg-brand-soft text-brand-dark">
              <item.icon className="h-6 w-6" />
            </div>
            <h2 className="mt-5 font-heading text-2xl font-semibold text-slate-950">
              {item.title}
            </h2>
            <p className="mt-3 text-sm leading-6 text-slate-600">
              {item.description}
            </p>
            <Link to={item.to} className="mt-6 inline-flex">
              <Button>
                Abrir
                <ShieldCheck className="h-4 w-4" />
              </Button>
            </Link>
          </Card>
        ))}
      </div>
    </div>
  );
}
