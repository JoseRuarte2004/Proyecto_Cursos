import { motion } from "framer-motion";
import { Outlet } from "react-router-dom";

import { Logo } from "@/components/shared/logo";

export function AuthLayout() {
  return (
    <div className="hero-glow relative min-h-screen overflow-hidden px-4 py-8">
      <div className="absolute inset-0">
        <div className="absolute left-[-10%] top-10 h-72 w-72 rounded-full bg-brand/20 blur-3xl" />
        <div className="absolute bottom-0 right-[-8%] h-80 w-80 rounded-full bg-accent/20 blur-3xl" />
      </div>
      <div className="relative mx-auto flex min-h-[calc(100vh-4rem)] max-w-6xl flex-col justify-between gap-10 lg:flex-row lg:items-center">
        <div className="max-w-xl">
          <Logo />
          <motion.h1
            className="mt-10 font-heading text-4xl font-semibold leading-tight text-slate-950 sm:text-5xl"
            initial={{ opacity: 0, y: 14 }}
            animate={{ opacity: 1, y: 0 }}
          >
            Aprender, enseñar y gestionar cursos desde una sola experiencia.
          </motion.h1>
          <motion.p
            className="mt-5 max-w-lg text-base text-slate-600 sm:text-lg"
            initial={{ opacity: 0, y: 14 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ delay: 0.05 }}
          >
            Un onboarding claro, checkout listo para enchufar pagos reales y
            paneles optimizados para cada rol.
          </motion.p>
        </div>
        <motion.div
          className="w-full max-w-md"
          initial={{ opacity: 0, y: 24, scale: 0.98 }}
          animate={{ opacity: 1, y: 0, scale: 1 }}
        >
          <Outlet />
        </motion.div>
      </div>
    </div>
  );
}
