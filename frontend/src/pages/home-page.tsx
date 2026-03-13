import { useInfiniteQuery } from "@tanstack/react-query";
import { Filter, Search, Sparkles } from "lucide-react";
import { useMemo, useState } from "react";
import { Link } from "react-router-dom";

import { coursesApi } from "@/api/endpoints";
import { useSession } from "@/auth/session";
import { FloatingAssistant } from "@/features/courses/assistant/floating-assistant";
import { CourseGrid } from "@/features/courses/components";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Select } from "@/components/ui/select";
import { PageIntro } from "@/components/shared/page-intro";
import {
  EmptyState,
  ErrorState,
  LoadingSkeleton,
  SectionMotion,
} from "@/components/shared/feedback";

const PAGE_SIZE = 9;

export function HomePage() {
  const { user } = useSession();
  const [search, setSearch] = useState("");
  const [category, setCategory] = useState("all");

  const query = useInfiniteQuery({
    queryKey: ["catalog", PAGE_SIZE],
    queryFn: ({ pageParam = 0 }) => coursesApi.listPublished(PAGE_SIZE, pageParam),
    initialPageParam: 0,
    getNextPageParam: (lastPage, _pages, lastOffset) =>
      lastPage.length < PAGE_SIZE ? undefined : lastOffset + PAGE_SIZE,
  });

  const courses = useMemo(
    () => query.data?.pages.flatMap((page) => page) ?? [],
    [query.data],
  );

  const categories = useMemo(() => {
    const set = new Set(courses.map((course) => course.category));
    return ["all", ...Array.from(set)];
  }, [courses]);

  const filteredCourses = useMemo(() => {
    return courses.filter((course) => {
      const matchesSearch =
        !search ||
        [course.title, course.description, course.category]
          .join(" ")
          .toLowerCase()
          .includes(search.toLowerCase());
      const matchesCategory = category === "all" || course.category === category;
      return matchesSearch && matchesCategory;
    });
  }, [category, courses, search]);

  return (
    <>
      <div className="page-shell space-y-8">
      <SectionMotion>
        <PageIntro
          eyebrow="SaaS Learning"
          title="Un catálogo listo para comprar y entrar a estudiar sin fricción."
          description="Explorá cursos publicados, compará categorías y arrancá el flujo de reserva y checkout desde una interfaz clara y liviana."
          actions={
            <div className="flex flex-wrap gap-3">
              <div className="rounded-2xl border border-brand/10 bg-brand-soft px-4 py-3 text-sm text-brand-dark">
                <span className="font-semibold">{courses.length}</span> cursos cargados
              </div>
              <div className="rounded-2xl border border-emerald-100 bg-emerald-50 px-4 py-3 text-sm text-emerald-700">
                Checkout real listo para redirigir a Mercado Pago
              </div>
            </div>
          }
        />
      </SectionMotion>

      <section className="grid gap-5 lg:grid-cols-[1.35fr_0.65fr]">
        <div className="hero-glow rounded-[32px] border border-white/70 p-6 shadow-card sm:p-8">
          <div className="flex h-full flex-col justify-between gap-6">
            <div>
              <div className="inline-flex items-center gap-2 rounded-full bg-white/80 px-3 py-2 text-xs font-semibold uppercase tracking-[0.24em] text-brand">
                <Sparkles className="h-4 w-4" />
                Front listo para vender
              </div>
              <h2 className="mt-5 max-w-2xl font-heading text-3xl font-semibold leading-tight text-slate-950 sm:text-4xl">
                Cursos con diseño premium, estados claros y acceso inmediato a las clases.
              </h2>
              <p className="mt-4 max-w-2xl text-sm leading-6 text-slate-600 sm:text-base">
                El front detecta sesión, protege rutas por rol y te acompaña desde el
                catálogo hasta el aula, incluyendo la transición por checkout real.
              </p>
            </div>
            <div className="flex flex-wrap gap-3">
              <Link to={user ? "/me/courses" : "/register"}>
                <Button size="lg">
                  {user ? "Ir a mis cursos" : "Crear cuenta y empezar"}
                </Button>
              </Link>
              {!user ? (
                <Link to="/login">
                  <Button variant="secondary" size="lg">
                    Ya tengo cuenta
                  </Button>
                </Link>
              ) : null}
            </div>
          </div>
        </div>
        <div className="glass-panel rounded-[32px] p-6 shadow-card">
          <div className="flex items-center gap-2 text-sm font-semibold uppercase tracking-[0.2em] text-slate-500">
            <Filter className="h-4 w-4" />
            Explorar
          </div>
          <div className="mt-6 space-y-4">
            <div className="relative">
              <Search className="pointer-events-none absolute left-4 top-1/2 h-4 w-4 -translate-y-1/2 text-slate-400" />
              <Input
                className="pl-11"
                placeholder="Buscar por título, categoría o tema"
                value={search}
                onChange={(event) => setSearch(event.target.value)}
              />
            </div>
            <Select value={category} onChange={(event) => setCategory(event.target.value)}>
              {categories.map((option) => (
                <option key={option} value={option}>
                  {option === "all" ? "Todas las categorías" : option}
                </option>
              ))}
            </Select>
          </div>
        </div>
      </section>

      {query.isLoading ? (
        <div className="grid gap-5 sm:grid-cols-2 xl:grid-cols-3">
          {Array.from({ length: 6 }).map((_, index) => (
            <LoadingSkeleton key={index} className="h-[390px]" lines={8} />
          ))}
        </div>
      ) : query.isError ? (
        <ErrorState
          description={query.error.message}
          action={<Button onClick={() => query.refetch()}>Reintentar</Button>}
        />
      ) : filteredCourses.length ? (
        <>
          <CourseGrid courses={filteredCourses} />
          <div className="flex justify-center">
            {query.hasNextPage ? (
              <Button
                variant="secondary"
                size="lg"
                onClick={() => query.fetchNextPage()}
                loading={query.isFetchingNextPage}
              >
                Cargar más cursos
              </Button>
            ) : (
              <div className="rounded-full bg-white/80 px-4 py-2 text-sm text-slate-500 shadow-card">
                Ya viste todo el catálogo publicado.
              </div>
            )}
          </div>
        </>
      ) : (
        <EmptyState
          title="No encontramos cursos con ese filtro"
          description="Probá con otra categoría o cambiá la búsqueda para ver más resultados."
        />
      )}
      </div>
      <FloatingAssistant />
    </>
  );
}
