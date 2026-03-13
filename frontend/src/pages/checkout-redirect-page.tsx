import { useQuery } from "@tanstack/react-query";
import { ExternalLink, LoaderCircle } from "lucide-react";
import { useEffect, useMemo } from "react";
import { useSearchParams } from "react-router-dom";

import { paymentsApi } from "@/api/endpoints";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";

function isAllowedMercadoPagoCheckout(urlValue: string) {
  try {
    const parsed = new URL(urlValue);
    const host = parsed.hostname.toLowerCase();
    const allowedSuffixes = [
      "mercadopago.com",
      "mercadopago.com.ar",
      "mercadopago.com.br",
      "mercadopago.cl",
      "mercadopago.com.mx",
      "mpago.la",
    ];

    return allowedSuffixes.some(
      (suffix) => host === suffix || host.endsWith(`.${suffix}`),
    );
  } catch {
    return false;
  }
}

export function CheckoutRedirectPage() {
  const [searchParams] = useSearchParams();
  const orderId = searchParams.get("orderId") ?? "";

  const orderQuery = useQuery({
    queryKey: ["checkout-redirect-order", orderId],
    queryFn: () => paymentsApi.getOrder(orderId),
    enabled: Boolean(orderId),
    staleTime: 5_000,
  });

  const checkoutUrl = useMemo(() => {
    const candidate = orderQuery.data?.checkoutUrl?.trim() ?? "";
    if (!candidate || !isAllowedMercadoPagoCheckout(candidate)) {
      return "";
    }
    return candidate;
  }, [orderQuery.data?.checkoutUrl]);

  useEffect(() => {
    if (!checkoutUrl) {
      return;
    }

    const timer = window.setTimeout(() => {
      window.location.assign(checkoutUrl);
    }, 1200);

    return () => window.clearTimeout(timer);
  }, [checkoutUrl]);

  const loading = orderQuery.isLoading;

  return (
    <div className="page-shell flex min-h-[80vh] items-center justify-center">
      <Card className="max-w-2xl p-8 text-center sm:p-10">
        <div className="mx-auto flex h-16 w-16 items-center justify-center rounded-3xl bg-brand-soft text-brand-dark">
          <LoaderCircle className="h-8 w-8 animate-spin" />
        </div>
        <h1 className="mt-6 font-heading text-3xl font-semibold text-slate-950">
          Te estamos llevando a Mercado Pago...
        </h1>
        <p className="mx-auto mt-3 max-w-xl text-sm leading-6 text-slate-600">
          La orden ya fue creada y el checkout se consulta nuevamente desde el backend
          antes de salir a Mercado Pago.
        </p>

        {loading ? (
          <div className="mt-8 rounded-2xl bg-slate-100 px-4 py-3 text-sm text-slate-600">
            Preparando checkout para la orden <strong>{orderId}</strong>.
          </div>
        ) : (
          <div className="mt-8 space-y-4">
            <div className="rounded-[28px] border border-dashed border-brand/30 bg-brand-soft/60 p-5 text-left">
              <p className="text-sm font-semibold text-brand-dark">
                {checkoutUrl
                  ? "No pudimos redirigir automaticamente"
                  : "No pudimos preparar el checkout"}
              </p>
              <p className="mt-2 text-sm text-slate-700">
                {checkoutUrl
                  ? "Abri el checkout manualmente con el boton de abajo."
                  : orderQuery.error instanceof Error
                    ? orderQuery.error.message
                    : "La orden no devolvio una URL de checkout valida."}
              </p>
            </div>
            <a href={checkoutUrl || "#"} className="inline-flex">
              <Button size="lg" disabled={!checkoutUrl}>
                Abrir Mercado Pago
                <ExternalLink className="h-4 w-4" />
              </Button>
            </a>
          </div>
        )}
      </Card>
    </div>
  );
}
