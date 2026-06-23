import { type ZodErrorMap, ZodIssueCode, ZodParsedType } from "zod";

import i18n from "./i18n";

// Global Zod error map: the single source of client-side validation copy. Every
// branch resolves to a `validation.*` i18n key, read live via the default i18n
// instance so messages follow the active language at the moment validation runs
// (already-displayed errors re-translate on the next validate after a language
// switch — acceptable). Returns `{ message }` per the Zod v3 errorMap contract.
//
// Schemas therefore declare plain rules (`z.string().min(1)`, `.email()`, …) with
// NO inline message. The only exception is a custom `.refine`, which carries its
// key via `{ params: { i18n: "validation.foo" } }` (see the `custom` branch).
const t = (key: string, opts?: Record<string, unknown>) => i18n.t(key, opts);

export const zodErrorMap: ZodErrorMap = (issue, ctx) => {
  switch (issue.code) {
    case ZodIssueCode.too_small: {
      if (issue.type === "string") {
        return {
          message:
            Number(issue.minimum) <= 1
              ? t("validation.required")
              : t("validation.minLength", { count: Number(issue.minimum) }),
        };
      }
      if (issue.type === "array" || issue.type === "set") {
        return { message: t("validation.required") };
      }
      // number | bigint | date
      return { message: t("validation.minValue", { min: String(issue.minimum) }) };
    }
    case ZodIssueCode.too_big: {
      if (issue.type === "string") {
        return { message: t("validation.maxLength", { count: Number(issue.maximum) }) };
      }
      return { message: t("validation.maxValue", { max: String(issue.maximum) }) };
    }
    case ZodIssueCode.invalid_string: {
      if (issue.validation === "email") return { message: t("validation.email") };
      return { message: t("validation.invalid") };
    }
    case ZodIssueCode.invalid_type: {
      // A missing/NaN value on an otherwise-required field reads as "required".
      if (
        issue.received === ZodParsedType.undefined ||
        issue.received === ZodParsedType.null ||
        issue.received === ZodParsedType.nan
      ) {
        return { message: t("validation.required") };
      }
      if (issue.expected === "number" || issue.expected === "bigint") {
        return { message: t("validation.number") };
      }
      return { message: t("validation.invalid") };
    }
    case ZodIssueCode.invalid_enum_value:
      return { message: t("validation.invalidOption") };
    case ZodIssueCode.custom: {
      // Custom refine messages carry their i18n key in params.i18n.
      const key = (issue.params as { i18n?: string } | undefined)?.i18n;
      if (key) return { message: t(key) };
      return { message: ctx.defaultError };
    }
    default:
      return { message: ctx.defaultError };
  }
};
