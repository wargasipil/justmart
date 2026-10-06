import { Field, HStack, Stack, Text } from "@chakra-ui/react";
import { useTranslation } from "react-i18next";
import { Controller, type Control } from "react-hook-form";

import EnumSelect from "../../components/EnumSelect";
import NumberInput from "../../components/NumberInput";
import { ExpiryDefault } from "../../gen/inventory_iface/v1/product_pb";
import type { FormValues } from "./ProductFormFields";

// How this product's new lots get their expiry pre-filled on the Receive
// screen. Page-local to the product form (its only caller). The server stores
// the setting; lib/expiry.ts computes the date when receiving.

/** The setting as one line of text, for the product detail page. */
export function expiryDefaultSummary(
  t: (k: string, o?: Record<string, unknown>) => string,
  p: { expiryDefault: ExpiryDefault; expiryDefaultMonths: number },
): string {
  if (p.expiryDefault === ExpiryDefault.MONTHS && p.expiryDefaultMonths > 0) {
    return t("inventory.products.expiryDefaultMonthsValue", { count: p.expiryDefaultMonths });
  }
  if (p.expiryDefault === ExpiryDefault.NONE) return t("inventory.products.expiryDefaultNone");
  return t("inventory.products.expiryDefaultManual");
}

export default function ProductExpiryDefaultField({
  control,
  mode,
  isPharmacy,
  prescriptionRequired,
}: {
  control: Control<FormValues>;
  mode: ExpiryDefault;
  isPharmacy: boolean;
  prescriptionRequired: boolean;
}) {
  const { t } = useTranslation();
  const options = [
    { value: String(ExpiryDefault.MANUAL), label: t("inventory.products.expiryDefaultManual") },
    { value: String(ExpiryDefault.MONTHS), label: t("inventory.products.expiryDefaultMonths") },
    { value: String(ExpiryDefault.NONE), label: t("inventory.products.expiryDefaultNone") },
  ];
  // UNSPECIFIED reads as MANUAL — what every product did before the setting.
  const shown = mode === ExpiryDefault.UNSPECIFIED ? ExpiryDefault.MANUAL : mode;

  return (
    <Stack gap={2}>
      <HStack gap={3} align="start" wrap="wrap">
        <Controller
          control={control}
          name="expiryDefault"
          render={({ field }) => (
            <Field.Root flex="1 1 14rem">
              <Field.Label>{t("inventory.products.expiryDefault")}</Field.Label>
              <EnumSelect
                value={String(shown)}
                onChange={(v) => field.onChange(Number(v))}
                items={options}
                itemToString={(o) => o.label}
                itemToValue={(o) => o.value}
                width="full"
              />
            </Field.Root>
          )}
        />
        {shown === ExpiryDefault.MONTHS && (
          <Controller
            control={control}
            name="expiryDefaultMonths"
            render={({ field, fieldState }) => (
              <Field.Root width="7rem" required invalid={!!fieldState.error}>
                <Field.Label>
                  {t("inventory.products.expiryDefaultMonthsLabel")}
                  <Field.RequiredIndicator />
                </Field.Label>
                <NumberInput value={field.value} onChange={field.onChange} onBlur={field.onBlur} max={120} />
                {fieldState.error && <Field.ErrorText>{fieldState.error.message}</Field.ErrorText>}
              </Field.Root>
            )}
          />
        )}
      </HStack>
      {shown === ExpiryDefault.MONTHS && (
        <Text fontSize="xs" color="fg.muted">
          {t("inventory.products.expiryDefaultHelp")}
        </Text>
      )}
      {/* The pharmacy rule, said where the setting is made rather than
          discovered at the receive screen: a prescription medicine is never
          pre-filled, whatever is chosen here. */}
      {isPharmacy && prescriptionRequired && shown !== ExpiryDefault.MANUAL && (
        <Text fontSize="xs" color="fg.warning">
          {t("inventory.products.expiryDefaultRxNote")}
        </Text>
      )}
    </Stack>
  );
}
