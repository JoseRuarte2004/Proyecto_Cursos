import { zodResolver } from "@hookform/resolvers/zod";
import { useMutation } from "@tanstack/react-query";
import { Sparkles } from "lucide-react";
import { useState } from "react";
import { useForm } from "react-hook-form";
import { Link, useNavigate } from "react-router-dom";
import { toast } from "sonner";
import { z } from "zod";

import { usersApi } from "@/api/endpoints";
import { useSession } from "@/auth/session";
import { Card } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { FormField } from "@/components/shared/form-field";

const registerSchema = z.object({
  name: z.string().min(2, "Ingresa tu nombre."),
  email: z.string().email("Ingresa un email valido."),
  password: z.string().min(8, "La contrasena debe tener al menos 8 caracteres."),
  phone: z.string().optional(),
  dni: z.string().optional(),
  address: z.string().optional(),
});

const verifySchema = z.object({
  code: z.string().regex(/^\d{6}$/, "Ingresa un codigo de 6 digitos."),
});

type RegisterFormValues = z.infer<typeof registerSchema>;
type VerifyFormValues = z.infer<typeof verifySchema>;

type PendingVerification = {
  email: string;
  password: string;
};

export function RegisterPage() {
  const navigate = useNavigate();
  const { login } = useSession();
  const [pendingVerification, setPendingVerification] =
    useState<PendingVerification | null>(null);

  const registerForm = useForm<RegisterFormValues>({
    resolver: zodResolver(registerSchema),
    defaultValues: {
      name: "",
      email: "",
      password: "",
      phone: "",
      dni: "",
      address: "",
    },
  });

  const verifyForm = useForm<VerifyFormValues>({
    resolver: zodResolver(verifySchema),
    defaultValues: {
      code: "",
    },
  });

  const registerMutation = useMutation({
    mutationFn: (values: RegisterFormValues) => usersApi.registerWithCode(values),
    onSuccess: (_, values) => {
      setPendingVerification({
        email: values.email,
        password: values.password,
      });
      verifyForm.reset({ code: "" });
      toast.success(
        "Cuenta creada. Revisa el codigo en el log de users-api y verificá tu email.",
      );
    },
    onError: (error: Error) => {
      toast.error(error.message);
    },
  });

  const verifyMutation = useMutation({
    mutationFn: async (values: VerifyFormValues) => {
      if (!pendingVerification) {
        throw new Error("Primero crea tu cuenta.");
      }

      await usersApi.verifyCode({
        email: pendingVerification.email,
        code: values.code,
      });

      return usersApi.login({
        email: pendingVerification.email,
        password: pendingVerification.password,
      });
    },
    onSuccess: (result) => {
      login(result.token, result.user);
      toast.success("Email verificado. Ya puedes ingresar.");
      navigate("/home", { replace: true });
    },
    onError: (error: Error) => {
      toast.error(error.message);
    },
  });

  return (
    <Card className="p-6 sm:p-8">
      <div className="mb-8">
        <div className="flex h-12 w-12 items-center justify-center rounded-2xl bg-accent/10 text-accent">
          <Sparkles className="h-6 w-6" />
        </div>
        <h2 className="mt-5 font-heading text-3xl font-semibold text-slate-950">
          {pendingVerification ? "Verifica tu email" : "Crea tu cuenta"}
        </h2>
        <p className="mt-2 text-sm text-slate-600">
          {pendingVerification
            ? `Te registraste con ${pendingVerification.email}. Ingresa el codigo de 6 digitos.`
            : "Empieza con un perfil de alumno. Despues un admin puede promoverte a docente."}
        </p>
      </div>

      {!pendingVerification ? (
        <form
          className="space-y-4"
          onSubmit={registerForm.handleSubmit((values) =>
            registerMutation.mutate(values),
          )}
        >
          <FormField label="Nombre" error={registerForm.formState.errors.name?.message}>
            <Input placeholder="Ana Martinez" {...registerForm.register("name")} />
          </FormField>
          <FormField label="Email" error={registerForm.formState.errors.email?.message}>
            <Input
              type="email"
              placeholder="ana@ejemplo.com"
              {...registerForm.register("email")}
            />
          </FormField>
          <FormField
            label="Contrasena"
            error={registerForm.formState.errors.password?.message}
          >
            <Input
              type="password"
              placeholder="Minimo 8 caracteres"
              {...registerForm.register("password")}
            />
          </FormField>
          <div className="grid gap-4 sm:grid-cols-2">
            <FormField label="Telefono">
              <Input placeholder="11 5555 5555" {...registerForm.register("phone")} />
            </FormField>
            <FormField label="DNI">
              <Input placeholder="30111222" {...registerForm.register("dni")} />
            </FormField>
          </div>
          <FormField label="Direccion">
            <Input
              placeholder="Calle 123, Ciudad"
              {...registerForm.register("address")}
            />
          </FormField>
          <Button
            type="submit"
            className="w-full"
            size="lg"
            loading={registerMutation.isPending}
          >
            Crear cuenta
          </Button>
        </form>
      ) : (
        <form
          className="space-y-4"
          onSubmit={verifyForm.handleSubmit((values) => verifyMutation.mutate(values))}
        >
          <FormField label="Codigo de verificacion" error={verifyForm.formState.errors.code?.message}>
            <Input placeholder="459812" maxLength={6} {...verifyForm.register("code")} />
          </FormField>
          <Button
            type="submit"
            className="w-full"
            size="lg"
            loading={verifyMutation.isPending}
          >
            Verificar email
          </Button>
          <Button
            type="button"
            variant="secondary"
            className="w-full"
            onClick={() => setPendingVerification(null)}
          >
            Cambiar datos de registro
          </Button>
        </form>
      )}

      <p className="mt-6 text-center text-sm text-slate-600">
        Ya tienes cuenta?{" "}
        <Link to="/login" className="font-semibold text-brand">
          Ingresa aqui
        </Link>
      </p>
    </Card>
  );
}
