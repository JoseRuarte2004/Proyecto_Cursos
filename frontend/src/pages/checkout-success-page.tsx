import { useQuery } from "@tanstack/react-query";
import { CheckCircle2, LoaderCircle } from "lucide-react";
import { useEffect } from "react";
import { Link, useNavigate, useSearchParams } from "react-router-dom";

import { enrollmentsApi, paymentsApi } from "@/api/endpoints";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";

export function CheckoutSuccessPage() {
  const [searchParams] = useSearchParams();
  const navigate = useNavigate();
  const courseId = searchParams.get("courseId") ?? "";
  const orderId = searchParams.get("orderId") ?? "";
  const paymentId =
    searchParams.get("payment_id") ??
    searchParams.get("collection_id") ??
    "";

  const orderQuery = useQuery({
    queryKey: ["checkout-order", orderId, paymentId],
    queryFn: () =>
      paymentsApi.getOrder(orderId, {
        paymentId,
      }),
    enabled: Boolean(orderId),
    refetchInterval: (query) => {
      const status = query.state.data?.status;
      if (!status || status === "created" || status === "pending") {
        return 2000;
      }
      return false;
    },
  });

  const enrollmentQuery = useQuery({
    queryKey: ["post-checkout-enrollments", courseId],
    queryFn: () => enrollmentsApi.myEnrollments(),
    enabled: Boolean(courseId) && orderQuery.data?.status === "paid",
    refetchInterval: (query) => {
      const items = query.state.data;
      const active = items?.find(
        (item) => item.courseId === courseId && item.status === "active",
      );
      return active ? false : 1500;
    },
  });

  const order = orderQuery.data;
  const enrollment = enrollmentQuery.data?.find((item) => item.courseId === courseId);
  const active = enrollment?.status === "active";
  const isConfirming = !order || order.status === "created" || order.status === "pending";

  useEffect(() => {
    if (!active || !courseId) {
      return;
    }

    const timeoutId = window.setTimeout(() => {
      navigate(`/courses/${courseId}/classroom`, { replace: true });
    }, 250);

    return () => window.clearTimeout(timeoutId);
  }, [active, courseId, navigate]);

  return (
    <div className="page-shell flex min-h-[80vh] items-center justify-center">
      <Card className="max-w-2xl p-8 text-center sm:p-10">
        <div className="mx-auto flex h-16 w-16 items-center justify-center rounded-3xl bg-emerald-100 text-emerald-700">
          {isConfirming ? (
            <LoaderCircle className="h-8 w-8 animate-spin" />
          ) : (
            <CheckCircle2 className="h-8 w-8" />
          )}
        </div>
        <h1 className="mt-6 font-heading text-3xl font-semibold text-slate-950">
          {active
            ? "Pago confirmado"
            : isConfirming
              ? "Estamos confirmando tu pago"
              : "Pago confirmado"}
        </h1>
        <p className="mx-auto mt-3 max-w-xl text-sm leading-6 text-slate-600">
          {active
            ? "La inscripcion ya quedo activa. Te estamos llevando directo al curso."
            : isConfirming
              ? "Mercado Pago ya te devolvio a la plataforma. Ahora esperamos la confirmacion del webhook real para actualizar la orden."
              : "La orden ya quedo acreditada. Ahora seguimos esperando la activacion final de la inscripcion."}
        </p>
        <div className="mt-6 rounded-2xl bg-slate-100 px-4 py-3 text-sm text-slate-700">
          Estado de la orden: <strong>{order?.status ?? "sin confirmar"}</strong>
          {order?.providerStatus ? ` (${order.providerStatus})` : ""}
        </div>
        <div className="mt-3 rounded-2xl bg-slate-100 px-4 py-3 text-sm text-slate-700">
          Estado de la inscripcion: <strong>{enrollment?.status ?? "pendiente"}</strong>
        </div>
        <div className="mt-8 flex flex-col gap-3 sm:flex-row sm:justify-center">
          <Link to="/me/courses">
            <Button variant="secondary" size="lg">
              Ir a mis cursos
            </Button>
          </Link>
          {!active ? null : (
            <Link to={`/courses/${courseId}/classroom`}>
              <Button size="lg">Entrar al curso</Button>
            </Link>
          )}
        </div>
      </Card>
    </div>
  );
}
