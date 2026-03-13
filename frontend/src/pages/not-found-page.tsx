import { Link } from "react-router-dom";

import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";

export function NotFoundPage() {
  return (
    <div className="page-shell flex min-h-[70vh] items-center justify-center">
      <Card className="max-w-xl p-8 text-center">
        <p className="text-xs font-semibold uppercase tracking-[0.28em] text-brand">
          404
        </p>
        <h1 className="mt-3 font-heading text-4xl font-semibold text-slate-950">
          Esta ruta no existe
        </h1>
        <p className="mt-3 text-sm leading-6 text-slate-600">
          Volvé al catálogo o al panel correspondiente para seguir navegando.
        </p>
        <div className="mt-6 flex justify-center">
          <Link to="/home">
            <Button>Ir al inicio</Button>
          </Link>
        </div>
      </Card>
    </div>
  );
}
