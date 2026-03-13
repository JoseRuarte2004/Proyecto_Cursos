import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Search, ShieldPlus } from "lucide-react";
import { useMemo, useState } from "react";
import { toast } from "sonner";

import { usersApi } from "@/api/endpoints";
import type { Role } from "@/api/types";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Modal } from "@/components/ui/modal";
import { Select } from "@/components/ui/select";
import { DataTable } from "@/components/ui/table";
import { PageIntro } from "@/components/shared/page-intro";
import {
  EmptyState,
  ErrorState,
  LoadingSkeleton,
} from "@/components/shared/feedback";

export function AdminUsersPage() {
  const [search, setSearch] = useState("");
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [nextRole, setNextRole] = useState<Role>("teacher");
  const queryClient = useQueryClient();

  const usersQuery = useQuery({
    queryKey: ["admin-users"],
    queryFn: () => usersApi.listAdminUsers(),
  });

  const detailQuery = useQuery({
    queryKey: ["admin-user", selectedId],
    queryFn: () => usersApi.getAdminUser(selectedId ?? ""),
    enabled: Boolean(selectedId),
  });

  const changeRoleMutation = useMutation({
    mutationFn: ({ id, role }: { id: string; role: Role }) =>
      usersApi.changeRole(id, role),
    onSuccess: () => {
      toast.success("Rol actualizado.");
      void queryClient.invalidateQueries({ queryKey: ["admin-users"] });
      if (selectedId) {
        void queryClient.invalidateQueries({ queryKey: ["admin-user", selectedId] });
      }
    },
    onError: (error: Error) => {
      toast.error(error.message);
    },
  });

  const filteredUsers = useMemo(() => {
    const data = usersQuery.data ?? [];
    if (!search) {
      return data;
    }
    return data.filter((user) =>
      [user.name, user.email, user.role, user.id]
        .join(" ")
        .toLowerCase()
        .includes(search.toLowerCase()),
    );
  }, [search, usersQuery.data]);

  return (
    <div className="space-y-8">
      <PageIntro
        eyebrow="Admin / Usuarios"
        title="Usuarios, roles y datos sensibles."
        description="Podés listar usuarios, inspeccionar datos sensibles y cambiar el rol para asignar docentes."
      />

      <div className="rounded-[28px] bg-white/85 p-5 shadow-card">
        <div className="relative max-w-md">
          <Search className="pointer-events-none absolute left-4 top-1/2 h-4 w-4 -translate-y-1/2 text-slate-400" />
          <Input
            className="pl-11"
            placeholder="Buscar por nombre, email, rol o ID"
            value={search}
            onChange={(event) => setSearch(event.target.value)}
          />
        </div>
      </div>

      {usersQuery.isLoading ? (
        <LoadingSkeleton className="h-72" lines={8} />
      ) : usersQuery.isError ? (
        <ErrorState description={usersQuery.error.message} />
      ) : filteredUsers.length ? (
        <DataTable
          columns={["Nombre", "Email", "Rol", "Actualizado", "Acciones"]}
          rows={filteredUsers.map((user) => [
            <div key={`${user.id}-name`}>
              <p className="font-semibold text-slate-950">{user.name}</p>
              <p className="text-xs text-slate-400">{user.id}</p>
            </div>,
            user.email,
            <Badge key={`${user.id}-role`} tone="brand">
              {user.role}
            </Badge>,
            user.updatedAt,
            <Button key={`${user.id}-view`} variant="secondary" onClick={() => {
              setSelectedId(user.id);
              setNextRole(user.role === "teacher" ? "student" : "teacher");
            }}>
              Ver detalle
            </Button>,
          ])}
        />
      ) : (
        <EmptyState
          title="No hay usuarios para ese filtro"
          description="Probá otra búsqueda."
        />
      )}

      <Modal
        open={Boolean(selectedId)}
        title="Detalle de usuario"
        description="Esta vista usa el endpoint sensible del backend y permite cambiar rol."
        onClose={() => setSelectedId(null)}
      >
        {detailQuery.isLoading ? (
          <LoadingSkeleton lines={6} />
        ) : detailQuery.isError || !detailQuery.data ? (
          <ErrorState description={detailQuery.error?.message ?? "No pudimos cargar el usuario."} />
        ) : (
          <div className="space-y-6">
            <div className="grid gap-4 sm:grid-cols-2">
              <Detail label="Nombre" value={detailQuery.data.name} />
              <Detail label="Email" value={detailQuery.data.email} />
              <Detail label="Rol actual" value={detailQuery.data.role} />
              <Detail label="Teléfono" value={detailQuery.data.phone || "-"} />
              <Detail label="DNI" value={detailQuery.data.dni || "-"} />
              <Detail label="Dirección" value={detailQuery.data.address || "-"} />
            </div>
            <div className="rounded-[28px] border border-slate-200 p-4">
              <p className="text-sm font-semibold text-slate-900">Cambiar rol</p>
              <div className="mt-4 flex flex-col gap-3 sm:flex-row">
                <Select
                  value={nextRole}
                  onChange={(event) => setNextRole(event.target.value as Role)}
                >
                  <option value="student">student</option>
                  <option value="teacher">teacher</option>
                  <option value="admin">admin</option>
                </Select>
                <Button
                  onClick={() =>
                    changeRoleMutation.mutate({
                      id: detailQuery.data.id,
                      role: nextRole,
                    })
                  }
                  loading={changeRoleMutation.isPending}
                >
                  <ShieldPlus className="h-4 w-4" />
                  Actualizar rol
                </Button>
              </div>
            </div>
          </div>
        )}
      </Modal>
    </div>
  );
}

function Detail({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-2xl bg-slate-50 p-4">
      <p className="text-xs font-semibold uppercase tracking-[0.24em] text-slate-400">
        {label}
      </p>
      <p className="mt-2 text-sm font-medium text-slate-900">{value}</p>
    </div>
  );
}
