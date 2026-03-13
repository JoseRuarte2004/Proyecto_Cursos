import { zodResolver } from "@hookform/resolvers/zod";
import { useMutation } from "@tanstack/react-query";
import { AlertTriangle, ArrowRight, KeyRound } from "lucide-react";
import { useMemo, useState } from "react";
import { useForm } from "react-hook-form";
import { Link, Navigate, useLocation, useNavigate } from "react-router-dom";
import { toast } from "sonner";
import { z } from "zod";

import { ApiError } from "@/api/client";
import { usersApi } from "@/api/endpoints";
import { useSession } from "@/auth/session";
import { Card } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { FormField } from "@/components/shared/form-field";

const schema = z.object({
  email: z.string().email("Ingresa un email valido."),
  password: z.string().min(8, "La contrasena debe tener al menos 8 caracteres."),
});

type FormValues = z.infer<typeof schema>;

const RECOVERY_THRESHOLD = 3;

function isInvalidCredentialsError(error: unknown): boolean {
  if (!(error instanceof ApiError)) {
    return false;
  }

  return (
    error.code === "INVALID_CREDENTIALS" ||
    error.message.toLowerCase().includes("invalid credentials")
  );
}

export function LoginPage() {
  const navigate = useNavigate();
  const location = useLocation();
  const { login, user, token } = useSession();
  const [failedAttempts, setFailedAttempts] = useState(0);
  const [autoRecoveryTriggered, setAutoRecoveryTriggered] = useState(false);

  const form = useForm<FormValues>({
    resolver: zodResolver(schema),
    defaultValues: {
      email: "",
      password: "",
    },
  });

  const typedEmail = form.watch("email").trim().toLowerCase();
  const canOfferRecovery = useMemo(
    () => failedAttempts >= RECOVERY_THRESHOLD,
    [failedAttempts],
  );

  const mutation = useMutation({
    mutationFn: usersApi.login,
    onSuccess: (result) => {
      setFailedAttempts(0);
      setAutoRecoveryTriggered(false);
      login(result.token, result.user);
      toast.success(`Bienvenido de nuevo, ${result.user.name}.`);

      const from = location.state?.from as string | undefined;
      if (from) {
        navigate(from, { replace: true });
        return;
      }

      navigate(
        result.user.role === "admin"
          ? "/admin"
          : result.user.role === "teacher"
            ? "/teacher"
            : "/home",
        { replace: true },
      );
    },
    onError: async (error: Error) => {
      toast.error(error.message);

      if (!isInvalidCredentialsError(error)) {
        return;
      }

      const nextFailedAttempts = failedAttempts + 1;
      setFailedAttempts(nextFailedAttempts);

      if (
        nextFailedAttempts >= RECOVERY_THRESHOLD &&
        !autoRecoveryTriggered &&
        z.string().email().safeParse(typedEmail).success
      ) {
        try {
          await usersApi.requestPasswordResetCode({ email: typedEmail });
          setAutoRecoveryTriggered(true);
          toast.info(
            "Detectamos varios intentos fallidos. Te enviamos un codigo para recuperar la contrasena.",
          );
        } catch (requestCodeError) {
          const message =
            requestCodeError instanceof Error
              ? requestCodeError.message
              : "No se pudo solicitar el codigo de recuperacion.";
          toast.error(message);
        }
      }
    },
  });

  if (user && token) {
    return <Navigate to="/home" replace />;
  }

  return (
    <Card className="p-6 sm:p-8">
      <div className="mb-8">
        <div className="flex h-12 w-12 items-center justify-center rounded-2xl bg-brand-soft text-brand-dark">
          <KeyRound className="h-6 w-6" />
        </div>
        <h2 className="mt-5 font-heading text-3xl font-semibold text-slate-950">
          Ingresa a tu cuenta
        </h2>
        <p className="mt-2 text-sm text-slate-600">
          Recupera tus cursos, gestiones o clases en segundos.
        </p>
      </div>

      <form
        className="space-y-5"
        onSubmit={form.handleSubmit((values) => mutation.mutate(values))}
      >
        <FormField label="Email" error={form.formState.errors.email?.message}>
          <Input type="email" placeholder="ana@ejemplo.com" {...form.register("email")} />
        </FormField>
        <FormField
          label="Contrasena"
          error={form.formState.errors.password?.message}
        >
          <Input
            type="password"
            placeholder="Tu contrasena"
            {...form.register("password")}
          />
        </FormField>

        <div className="text-right">
          <Link
            to={`/forgot-password${typedEmail ? `?email=${encodeURIComponent(typedEmail)}` : ""}`}
            className="text-sm font-semibold text-brand"
          >
            Olvidaste tu contrasena?
          </Link>
        </div>

        {canOfferRecovery ? (
          <div className="rounded-2xl border border-amber-200 bg-amber-50 px-4 py-3 text-sm text-amber-900">
            <div className="flex items-start gap-2">
              <AlertTriangle className="mt-0.5 h-4 w-4 flex-none" />
              <p>
                Detectamos varios intentos fallidos. Puedes recuperar el acceso con un
                codigo enviado a tu email.
              </p>
            </div>
            <Link
              to={`/forgot-password${typedEmail ? `?email=${encodeURIComponent(typedEmail)}` : ""}`}
              className="mt-3 inline-block font-semibold text-brand"
            >
              Recuperar contrasena
            </Link>
          </div>
        ) : null}

        <Button type="submit" className="w-full" size="lg" loading={mutation.isPending}>
          Entrar
          <ArrowRight className="h-4 w-4" />
        </Button>
      </form>

      <p className="mt-6 text-center text-sm text-slate-600">
        Todavia no tienes cuenta?{" "}
        <Link to="/register" className="font-semibold text-brand">
          Crear una ahora
        </Link>
      </p>
    </Card>
  );
}
