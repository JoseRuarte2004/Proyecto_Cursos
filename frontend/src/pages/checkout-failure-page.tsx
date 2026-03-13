import { useQuery } from "@tanstack/react-query";
import { AlertTriangle } from "lucide-react";
import { Link, useSearchParams } from "react-router-dom";

import { paymentsApi } from "@/api/endpoints";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";

export function CheckoutFailurePage() {
  const [searchParams] = useSearchParams();
  const courseId = searchParams.get("courseId") ?? "";
  const orderId = searchParams.get("orderId") ?? "";
  const paymentId =
    searchParams.get("payment_id") ??
    searchParams.get("collection_id") ??
    "";

  const orderQuery = useQuery({
    queryKey: ["checkout-failure-order", orderId, paymentId],
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

  const order = orderQuery.data;

  return (
    <div className="page-shell flex min-h-[80vh] items-center justify-center">
      <Card className="max-w-2xl p-8 text-center sm:p-10">
        <div className="mx-auto flex h-16 w-16 items-center justify-center rounded-3xl bg-amber-100 text-amber-700">
          <AlertTriangle className="h-8 w-8" />
        </div>
        <h1 className="mt-6 font-heading text-3xl font-semibold text-slate-950">
          El pago no se completo
        </h1>
        <p className="mx-auto mt-3 max-w-xl text-sm leading-6 text-slate-600">
          No se acredita ninguna inscripcion hasta que el pago quede aprobado.
          Podes volver al curso y reintentar cuando quieras.
        </p>
        {order ? (
          <div className="mt-6 rounded-2xl bg-slate-100 px-4 py-3 text-sm text-slate-700">
            Estado de la orden: <strong>{order.status}</strong>
            {order.providerStatus ? ` (${order.providerStatus})` : ""}
          </div>
        ) : null}
        <div className="mt-8 flex flex-col gap-3 sm:flex-row sm:justify-center">
          <Link to={`/courses/${courseId}`}>
            <Button size="lg">Reintentar</Button>
          </Link>
          <Link to="/home">
            <Button variant="secondary" size="lg">
              Volver al catalogo
            </Button>
          </Link>
        </div>
      </Card>
    </div>
  );
}
