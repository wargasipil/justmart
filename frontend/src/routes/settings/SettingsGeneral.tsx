import { Box, Button, Spinner, Stack } from "@chakra-ui/react";
import { zodResolver } from "@hookform/resolvers/zod";
import { useMemo } from "react";
import { useForm } from "react-hook-form";
import { useTranslation } from "react-i18next";
import { z } from "zod";

import FormField from "../../components/FormField";
import { useServerFormErrors } from "../../lib/formErrors";
import { toast } from "../../lib/toaster";
import { useSettingsQuery, useUpdateSettingsMutation } from "../../queries/settings";

const Schema = z.object({
  lowStockThreshold: z.coerce.number().int().min(0),
});
type FormValues = z.infer<typeof Schema>;

export default function SettingsGeneral() {
  const { t } = useTranslation();
  const q = useSettingsQuery();
  const save = useUpdateSettingsMutation();

  const values = useMemo<FormValues | undefined>(
    () => (q.data ? { lowStockThreshold: q.data.lowStockThreshold } : undefined),
    [q.data],
  );
  const form = useForm<FormValues>({ resolver: zodResolver(Schema), values });
  const onServerError = useServerFormErrors(form);

  const onSubmit = form.handleSubmit(async (v) => {
    try {
      await save.mutateAsync({ lowStockThreshold: v.lowStockThreshold });
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
