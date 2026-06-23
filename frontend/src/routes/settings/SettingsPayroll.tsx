import { useEffect, useState } from "react";
import { Box, Button, Spinner, Stack, Text, Textarea } from "@chakra-ui/react";
import { useTranslation } from "react-i18next";

import { usePayrollSettingsQuery, useSetPayrollSettingsMutation } from "../../queries/payroll";
import { toast } from "../../lib/toaster";

export default function SettingsPayroll() {
  const { t } = useTranslation();
  const q = usePayrollSettingsQuery();
  const save = useSetPayrollSettingsMutation();
  const [bpjs, setBpjs] = useState("");
  const [pph21, setPph21] = useState("");

  useEffect(() => {
    if (q.data) {
      setBpjs(q.data.bpjsConfigJson);
      setPph21(q.data.pph21ConfigJson);
    }
  }, [q.data]);

  const onSave = async () => {
    try {
      await save.mutateAsync({ bpjsConfigJson: bpjs, pph21ConfigJson: pph21 });
      toast.success(t("common.save") + " ✓");
    } catch {
      /* toast handled globally (payroll.config_invalid) */
    }
  };

  if (q.isLoading) {
    return (
      <Box p={8} textAlign="center">
        <Spinner />
      </Box>
    );
  }

  return (
    <Stack gap={4} maxW="3xl">
      <Box bg="orange.subtle" borderWidth="1px" borderColor="orange.muted" borderRadius="md" p={3}>
        <Text fontSize="sm">{t("payroll.settings.warning")}</Text>
      </Box>

      <Box>
        <Text fontSize="sm" fontWeight="medium" mb={1}>
          {t("payroll.settings.bpjs")}
        </Text>
        <Text fontSize="xs" color="fg.muted" mb={2}>
          {t("payroll.settings.bpjsHelp")}
        </Text>
        <Textarea value={bpjs} onChange={(e) => setBpjs(e.target.value)} rows={10} fontFamily="mono" fontSize="xs" />
      </Box>

      <Box>
        <Text fontSize="sm" fontWeight="medium" mb={1}>
          {t("payroll.settings.pph21")}
        </Text>
        <Text fontSize="xs" color="fg.muted" mb={2}>
          {t("payroll.settings.pph21Help")}
        </Text>
        <Textarea value={pph21} onChange={(e) => setPph21(e.target.value)} rows={14} fontFamily="mono" fontSize="xs" />
      </Box>

      <Box>
        <Button colorPalette="blue" onClick={onSave} loading={save.isPending}>
          {t("common.save")}
        </Button>
      </Box>
    </Stack>
  );
}
