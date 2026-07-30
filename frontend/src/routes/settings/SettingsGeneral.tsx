import { Box, Button, Field, Spinner, Stack } from "@chakra-ui/react";
import { zodResolver } from "@hookform/resolvers/zod";
import { useMemo } from "react";
import { Controller, useForm } from "react-hook-form";
import { useTranslation } from "react-i18next";
import { z } from "zod";

import EnumSelect from "../../components/EnumSelect";
import FormField from "../../components/FormField";
import { BussinessType } from "../../gen/settings_iface/v1/settings_pb";
import { useServerFormErrors } from "../../lib/formErrors";
import { toast } from "../../lib/toaster";
import { useSettingsQuery, useUpdateSettingsMutation } from "../../queries/settings";

// Business modes the owner can pick. UNSPECIFIED is never offered — it only
// exists as the "never configured" storage state (which behaves as retail).
const MODES = [BussinessType.RETAIL, BussinessType.PHARMACY_SHOP] as const;

const MAX_APP_TITLE_LEN = 60; // mirrors settings.MaxAppTitleLen on the backend

const Schema = z.object({
  lowStockThreshold: z.coerce.number().int().min(0),
  appTitle: z.string().max(MAX_APP_TITLE_LEN),
  businessType: z.coerce.number().int(),
});
type FormValues = z.infer<typeof Schema>;

export default function SettingsGeneral() {
  const { t } = useTranslation();
  const q = useSettingsQuery();
  const save = useUpdateSettingsMutation();

  const values = useMemo<FormValues | undefined>(
    () =>
      q.data
        ? {
            lowStockThreshold: q.data.lowStockThreshold,
            appTitle: q.data.appTitle,
            // A never-configured shop behaves as retail — show that in the picker
            // so saving doesn't look like a silent mode change.
            businessType:
              q.data.businessType === BussinessType.PHARMACY_SHOP
                ? BussinessType.PHARMACY_SHOP
                : BussinessType.RETAIL,
          }
        : undefined,
    [q.data],
  );
  const form = useForm<FormValues>({ resolver: zodResolver(Schema), values });
  const onServerError = useServerFormErrors(form);

  const modeLabel = (mode: BussinessType) =>
    mode === BussinessType.PHARMACY_SHOP
      ? t("settings.modes.pharmacy")
      : t("settings.modes.retail");

  const onSubmit = form.handleSubmit(async (v) => {
    try {
      await save.mutateAsync({
        lowStockThreshold: v.lowStockThreshold,
        appTitle: v.appTitle,
        businessType: v.businessType as BussinessType,
      });
      toast.success(t("common.save") + " ✓");
    } catch (err) {
      onServerError(err);
    }
  });

  if (q.isLoading) {
    return (
      <Box p={8} textAlign="center">
        <Spinner />
      </Box>
    );
  }

  return (
    <Stack gap={4} maxW="md">
      <FormField
        control={form.control}
        name="appTitle"
        label={t("settings.appTitle")}
        helperText={t("settings.appTitleHelp")}
        placeholder={t("app.name")}
      />
      <Controller
        control={form.control}
        name="businessType"
        render={({ field, fieldState }) => (
          <Field.Root invalid={!!fieldState.error}>
            <Field.Label>{t("settings.businessMode")}</Field.Label>
            <EnumSelect
              value={String(field.value)}
              onChange={(v) => field.onChange(Number(v))}
              items={MODES}
              itemToString={modeLabel}
              itemToValue={(m) => String(m)}
            />
            {fieldState.error ? (
              <Field.ErrorText>{fieldState.error.message}</Field.ErrorText>
            ) : (
              <Field.HelperText>{t("settings.businessModeHelp")}</Field.HelperText>
            )}
          </Field.Root>
        )}
      />
      <FormField
        control={form.control}
        name="lowStockThreshold"
        label={t("settings.lowStockThreshold")}
        helperText={t("settings.lowStockHelp")}
        number
      />
      <Box>
        <Button colorPalette="blue" onClick={onSubmit} loading={save.isPending}>
          {t("common.save")}
        </Button>
      </Box>
    </Stack>
  );
}
