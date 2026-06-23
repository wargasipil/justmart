import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import type { PartialMessage } from "@bufbuild/protobuf";

import { employeeClient } from "../lib/clients";
import type {
  ArchiveEmployeeRequest,
  CreateEmployeeRequest,
  UpdateEmployeeRequest,
} from "../gen/payroll_iface/v1/payroll_pb";

import { DEFAULT_PAGE_SIZE } from "../lib/pagination";

export type EmployeesQueryOpts = {
  includeInactive?: boolean;
  query?: string;
  page?: number;
  pageSize?: number;
};

export const employeeKeys = {
  all: ["employees"] as const,
  list: (opts: Required<EmployeesQueryOpts>) => [...employeeKeys.all, "list", opts] as const,
  one: (id: string) => [...employeeKeys.all, "one", id] as const,
};

export function useEmployeeQuery(id: string, enabled = true) {
  return useQuery({
    queryKey: employeeKeys.one(id),
    queryFn: async () => {
      const res = await employeeClient.getEmployee({ id });
      return res.employee;
    },
    enabled: enabled && !!id,
  });
}

// Server-paginated. Returns { rows, total }.
export function useEmployeesQuery(opts: EmployeesQueryOpts = {}) {
  const { includeInactive = false, query = "", page = 0, pageSize = DEFAULT_PAGE_SIZE } = opts;
  const q = useQuery({
    queryKey: employeeKeys.list({ includeInactive, query, page, pageSize }),
    queryFn: async () => {
      const res = await employeeClient.listEmployees({
        includeInactive,
        query,
        limit: pageSize,
        offset: page * pageSize,
      });
      return { rows: res.employees, total: res.total };
    },
  });
  return { ...q, rows: q.data?.rows ?? [], total: q.data?.total ?? 0 };
}

// Imperative search — call directly from <SearchableSelect loadOptions={...}>.
export async function searchEmployees(query: string) {
  const res = await employeeClient.searchEmployees({ query, limit: 20 });
  return res.employees;
}

export function useCreateEmployeeMutation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (req: PartialMessage<CreateEmployeeRequest>) => employeeClient.createEmployee(req),
    onSuccess: () => qc.invalidateQueries({ queryKey: employeeKeys.all }),
  });
}

export function useUpdateEmployeeMutation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (req: PartialMessage<UpdateEmployeeRequest>) => employeeClient.updateEmployee(req),
    onSuccess: () => qc.invalidateQueries({ queryKey: employeeKeys.all }),
  });
}

export function useArchiveEmployeeMutation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (req: PartialMessage<ArchiveEmployeeRequest>) => employeeClient.archiveEmployee(req),
    onSuccess: () => qc.invalidateQueries({ queryKey: employeeKeys.all }),
  });
}
