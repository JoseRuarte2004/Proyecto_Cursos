import { zodResolver } from "@hookform/resolvers/zod";
import { useMutation } from "@tanstack/react-query";
import { ArrowRight, KeyRound, Mail } from "lucide-react";
import { useMemo, useState } from "react";
import { useForm } from "react-hook-form";
import { Link, useNavigate, useSearchParams } from "react-router-dom";
import { toast } from "sonner";
import { z } from "zod";

import { usersApi } from "@/api/endpoints";
import { Card } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { FormField } from "@/components/shared/form-field";

const requestSchema = z.object({
  email: z.string().email("Ingresa un email valido."),
});

const resetSchema = z
  .object({
    code: z.string().regex(/^\d{6}$/, "Ingresa un codigo de 6 digitos."),
    newPassword: z.string().min(8, "La contrasena debe tener al menos 8 caracteres."),
    confirmPassword: z
      .string()
      .min(8, "La confirmacion debe tener al menos 8 caracteres."),
  })
  .refine((values) => values.newPassword === values.confirmPassword, {
    path: ["confirmPassword"],
    message: "Las contrasenas no coinciden.",
  });

type RequestFormValues = z.infer<typeof requestSchema>;
type ResetFormValues = z.infer<typeof resetSchema>;

export function ForgotPasswordPage() {
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const queryEmail = searchParams.get("email")?.trim() ?? "";

  const [emailForReset, setEmailForReset] = useState(queryEmail);
  const [isCodeRequested, setIsCodeRequested] = useState(false);

  const requestForm = useForm<RequestFormValues>({
    resolver: zodResolver(requestSchema),
    defaultValues: {
      email: queryEmail,
    },
  });

  const resetForm = useForm<ResetFormValues>({
    resolver: zodResolver(resetSchema),
    defaultValues: {
      code: "",
      newPassword: "",
      confirmPassword: "",
    },
  });

  const canShowResetStep = useMemo(
    () => isCodeRequested && emailForReset.length > 0,
    [isCodeRequested, emailForReset],
  );

  const requestCodeMutation = useMutation({
    mutationFn: usersApi.requestPasswordResetCode,
    onSuccess: (_, values) => {
      const normalizedEmail = values.email.trim().toLowerCase();
      setEmailForReset(normalizedEmail);
      setIsCodeRequested(true);
      resetForm.reset({
        code: "",
        newPassword: "",
        confirmPassword: "",
      });
      toast.success("Si el email existe, enviamos un codigo de recuperacion.");
    },
    onError: (error: Error) => {
      toast.error(error.message);
    },
  });

  const resetPasswordMutation = useMutation({
    mutationFn: (values: ResetFormValues) =>
      usersApi.resetPasswordWithCode({
        email: emailForReset,
        code: values.code,
        newPassword: values.newPassword,
      }),
    onSuccess: () => {
      toast.success("Contrasena actualizada. Ya puedes iniciar sesion.");
      navigate("/login", { replace: true });
    },
    onError: (error: Error) => {
      toast.error(error.message);
    },
  });

  return (
    <Card className="p-6 sm:p-8">
      <div className="mb-8">
        <div className="flex h-12 w-12 items-center justify-center rounded-2xl bg-brand-soft text-brand-dark">
          {canShowResetStep ? <KeyRound className="h-6 w-6" /> : <Mail className="h-6 w-6" />}
        </div>
        <h2 className="mt-5 font-heading text-3xl font-semibold text-slate-950">
          Recuperar contrasena
        </h2>
        <p className="mt-2 text-sm text-slate-600">
          {canShowResetStep
            ? `Ingresa el codigo enviado a ${emailForReset} y define tu nueva contrasena.`
            : "Te enviaremos un codigo de 6 digitos a tu email."}
        </p>
      </div>

      {!canShowResetStep ? (
        <form
          className="space-y-5"
          onSubmit={requestForm.handleSubmit((values) => requestCodeMutation.mutate(values))}
        >
          <FormField label="Email" error={requestForm.formState.errors.email?.message}>
            <Input
              type="email"
              placeholder="ana@ejemplo.com"
              {...requestForm.register("email")}
            />
          </FormField>
          <Button type="submit" className="w-full" size="lg" loading={requestCodeMutation.isPending}>
            Enviar codigo
            <ArrowRight className="h-4 w-4" />
          </Button>
        </form>
      ) : (
        <form
          className="space-y-5"
          onSubmit={resetForm.handleSubmit((values) => resetPasswordMutation.mutate(values))}
        >
          <FormField label="Codigo de recuperacion" error={resetForm.formState.errors.code?.message}>
            <Input placeholder="459812" maxLength={6} {...resetForm.register("code")} />
          </FormField>
          <FormField
            label="Nueva contrasena"
            error={resetForm.formState.errors.newPassword?.message}
          >
            <Input
              type="password"
              placeholder="Minimo 8 caracteres"
              {...resetForm.register("newPassword")}
            />
          </FormField>
          <FormField
            label="Repetir nueva contrasena"
            error={resetForm.formState.errors.confirmPassword?.message}
          >
            <Input
              type="password"
              placeholder="Repite la nueva contrasena"
              {...resetForm.register("confirmPassword")}
            />
          </FormField>
          <Button
            type="submit"
            className="w-full"
            size="lg"
            loading={resetPasswordMutation.isPending}
          >
            Actualizar contrasena
            <ArrowRight className="h-4 w-4" />
          </Button>

          <Button
            type="button"
            variant="ghost"
            className="w-full"
            onClick={() => {
              setIsCodeRequested(false);
              requestForm.reset({ email: emailForReset });
            }}
          >
            Enviar codigo de nuevo
          </Button>
        </form>
      )}

      <p className="mt-6 text-center text-sm text-slate-600">
        Volver al{" "}
        <Link to="/login" className="font-semibold text-brand">
          inicio de sesion
        </Link>
      </p>
    </Card>
  );
}
