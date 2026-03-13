import type { ReactNode } from "react";
import { useMutation, useQuery } from "@tanstack/react-query";
import { ArrowRight, BookOpenText, ShieldAlert, Users } from "lucide-react";
import { Link, useNavigate, useParams } from "react-router-dom";
import { toast } from "sonner";

import { coursesApi, enrollmentsApi, paymentsApi } from "@/api/endpoints";
import { ApiError } from "@/api/client";
import { useSession } from "@/auth/session";
import { buildCheckoutRedirectPath } from "@/features/enrollments/checkout";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { PageIntro } from "@/components/shared/page-intro";
import {
  ErrorState,
  LoaderScreen,
  SectionMotion,
} from "@/components/shared/feedback";
import { formatCurrency } from "@/app/utils";

type CheckoutActionResult =
  | {
      kind: "checkout";
      orderId: string;
      provider: "mercadopago" | "stripe";
    }
  | {
      kind: "classroom";
      courseTitle: string;
    };

export function CourseDetailPage() {
  const { id = "" } = useParams();
  const navigate = useNavigate();
  const { user } = useSession();

  const courseQuery = useQuery({
    queryKey: ["course", id],
    queryFn: () => coursesApi.getPublished(id),
    enabled: Boolean(id),
  });

  const availabilityQuery = useQuery({
    queryKey: ["availability", id],
    queryFn: () => enrollmentsApi.availability(id),
    enabled: Boolean(id),
  });

  async function getExistingEnrollment() {
    const enrollments = await enrollmentsApi.myEnrollments();
    return enrollments.find((item) => item.courseId === id) ?? null;
  }

  const checkoutMutation = useMutation({
    mutationFn: async (): Promise<CheckoutActionResult> => {
      let existingEnrollment: Awaited<ReturnType<typeof getExistingEnrollment>> = null;

      try {
        await enrollmentsApi.reserve(id);
      } catch (error) {
        if (!(error instanceof ApiError) || error.status !== 409) {
          throw error;
        }

        existingEnrollment = await getExistingEnrollment();
        if (existingEnrollment?.status === "active") {
          return {
            kind: "classroom",
            courseTitle: existingEnrollment.course.title,
          };
        }
        if (existingEnrollment?.status !== "pending") {
          throw error;
        }
      }

      try {
        const result = await paymentsApi.createOrder({
          courseId: id,
          provider: "mercadopago",
        });

        return {
          kind: "checkout",
          orderId: result.orderId,
          provider: result.provider,
        };
      } catch (error) {
        if (
          error instanceof ApiError &&
          error.status === 409 &&
          error.message === "pending enrollment is required"
        ) {
          existingEnrollment ??= await getExistingEnrollment();
          if (existingEnrollment?.status === "active") {
            return {
              kind: "classroom",
              courseTitle: existingEnrollment.course.title,
            };
          }
        }

        throw error;
      }
    },
    onSuccess: (result) => {
      if (result.kind === "classroom") {
        toast.info(
          `Ya estabas inscripto en ${result.courseTitle}. Te llevamos directo al curso.`,
        );
        navigate(`/courses/${id}/classroom`);
        return;
      }

      toast.success("Reserva creada. Te llevamos al checkout.");
      navigate(
        buildCheckoutRedirectPath({
          orderId: result.orderId,
          provider: result.provider,
          courseId: id,
        }),
      );
    },
    onError: (error: Error) => {
      toast.error(error.message);
    },
  });

  if (courseQuery.isLoading) {
    return <LoaderScreen label="Cargando detalle del curso..." />;
  }

  if (courseQuery.isError || !courseQuery.data) {
    return (
      <div className="page-shell">
        <ErrorState description={courseQuery.error?.message ?? "Curso no disponible."} />
      </div>
    );
  }

  const course = courseQuery.data;

  return (
    <div className="page-shell space-y-8">
      <SectionMotion>
        <PageIntro
          eyebrow="Detalle del curso"
          title={course.title}
          description={course.description}
          actions={
            <div className="flex flex-wrap gap-3">
              <Badge tone="brand">{course.category}</Badge>
              <Badge tone="success">{course.status}</Badge>
            </div>
          }
        />
      </SectionMotion>

      <section className="grid gap-8 lg:grid-cols-[1.2fr_0.8fr]">
        <Card className="overflow-hidden">
          <div
            className="h-72 bg-cover bg-center"
            style={{
              backgroundImage: `linear-gradient(180deg, rgba(15,23,42,0.04), rgba(15,23,42,0.34)), url(${course.imageUrl || "https://images.unsplash.com/photo-1498050108023-c5249f4df085?auto=format&fit=crop&w=1200&q=80"})`,
            }}
          />
          <div className="space-y-6 p-6 sm:p-8">
            <div className="grid gap-4 sm:grid-cols-2">
              <Metric label="Categoría" value={course.category} />
              <Metric
                label="Precio"
                value={formatCurrency(course.price, course.currency)}
              />
              <Metric label="Capacidad" value={`${course.capacity} alumnos`} />
              <Metric
                label="Disponibilidad"
                value={
                  availabilityQuery.data
                    ? `${availabilityQuery.data.available} lugares`
                    : "Consultando..."
                }
              />
            </div>
            <div className="rounded-[28px] border border-slate-200 bg-slate-50 p-5">
              <div className="flex items-start gap-3">
                <BookOpenText className="mt-0.5 h-5 w-5 text-brand" />
                <div>
                  <p className="font-semibold text-slate-900">Qué vas a encontrar</p>
                  <p className="mt-2 text-sm leading-6 text-slate-600">
                    Una experiencia de aula simple: sidebar de clases, reproductor
                    integrado y acceso por permisos según tu rol.
                  </p>
                </div>
              </div>
            </div>
          </div>
        </Card>

        <Card className="p-6 sm:p-8">
          <p className="text-sm font-semibold uppercase tracking-[0.24em] text-brand">
            Inscripción
          </p>
          <h2 className="mt-3 font-heading text-3xl font-semibold text-slate-950">
            Reservá tu lugar y seguí al checkout.
          </h2>
          <p className="mt-3 text-sm leading-6 text-slate-600">
            El backend crea la reserva, genera una orden interna y devuelve la URL real
            del checkout para continuar el pago en Mercado Pago.
          </p>

          <div className="mt-6 space-y-4">
            <InfoRow
              icon={<Users className="h-4 w-4" />}
              label="Capacidad total"
              value={`${course.capacity} cupos`}
            />
            <InfoRow
              icon={<ArrowRight className="h-4 w-4" />}
              label="Estado actual"
              value={course.status}
            />
            {availabilityQuery.data ? (
              <InfoRow
                icon={<ShieldAlert className="h-4 w-4" />}
                label="Lugares libres"
                value={`${availabilityQuery.data.available}`}
              />
            ) : null}
          </div>

          <div className="mt-8">
            {!user ? (
              <div className="space-y-3">
                <p className="text-sm text-slate-600">
                  Iniciá sesión para reservar y crear la orden.
                </p>
                <Link to="/login">
                  <Button className="w-full" size="lg">
                    Iniciá sesión para inscribirte
                  </Button>
                </Link>
              </div>
            ) : user.role !== "student" ? (
              <div className="rounded-2xl bg-slate-100 p-4 text-sm text-slate-600">
                La compra está habilitada solo para alumnos. Como {user.role}, podés
                recorrer el catálogo y entrar a tus vistas específicas.
              </div>
            ) : (
              <Button
                className="w-full"
                size="lg"
                loading={checkoutMutation.isPending}
                onClick={() => checkoutMutation.mutate()}
              >
                Inscribirme y pagar
              </Button>
            )}
          </div>
        </Card>
      </section>
    </div>
  );
}

function Metric({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-2xl border border-slate-200 bg-white p-4">
      <p className="text-xs font-semibold uppercase tracking-[0.24em] text-slate-400">
        {label}
      </p>
      <p className="mt-2 text-lg font-semibold text-slate-950">{value}</p>
    </div>
  );
}

function InfoRow({
  icon,
  label,
  value,
}: {
  icon: ReactNode;
  label: string;
  value: string;
}) {
  return (
    <div className="flex items-center justify-between gap-3 rounded-2xl border border-slate-200 px-4 py-3">
      <div className="flex items-center gap-3 text-sm text-slate-600">
        <span className="text-brand">{icon}</span>
        <span>{label}</span>
      </div>
      <span className="text-sm font-semibold text-slate-950">{value}</span>
    </div>
  );
}
