import type { FieldValues, UseFormReturn } from "react-hook-form";

import { applyServerError } from "./serverErrors";
import { toast } from "./toaster";

// useServerFormErrors returns an onError handler for a form mutation: it routes
// a known field-bearing backend token onto the offending form field, and falls
// back to a toast for everything else. This fallback is the guardrail for forms
// whose mutation opts into `meta: { silentError: true }` — without it a silenced
// error would vanish. Usage:
//   const onServerError = useServerFormErrors(form);
//   mut.mutate(payload, { onError: onServerError });   // or in a try/catch
export function useServerFormErrors<T extends FieldValues>(form: UseFormReturn<T>) {
  return (err: unknown) => {
    if (!applyServerError(err, form)) toast.fromError(err);
  };
}
