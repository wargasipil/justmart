import { Box, HStack, Spinner, Stack, Switch, Text } from "@chakra-ui/react";
import { useTranslation } from "react-i18next";

import { toast } from "../../lib/toaster";
import { useFeatureFlagsQuery, useSetFeatureFlagsMutation } from "../../queries/settings";

// Beta feature toggles. Each row flips a flag; the sidebar re-renders (show/hide
// gated menu items) as the featureFlags query invalidates — no reload.
export default function SettingsBeta() {
  const { t } = useTranslation();
  const q = useFeatureFlagsQuery();
  const save = useSetFeatureFlagsMutation();

  if (q.isLoading) {
    return (
      <Box p={8} textAlign="center">
        <Spinner />
      </Box>
    );
  }

  const payrollEnabled = q.data?.flags?.payrollEnabled ?? false;

  const setPayroll = async (next: boolean) => {
    try {
      await save.mutateAsync({ payrollEnabled: next });
    } catch (e) {
      toast.fromError(e);
    }
  };

  return (
    <Stack gap={4} maxW="lg">
      <Box borderWidth="1px" borderRadius="lg" p={4} bg="bg.subtle">
        <HStack justify="space-between" align="start" gap={4}>
          <Box>
            <Text fontWeight="medium">{t("settings.beta.enablePayroll")}</Text>
            <Text fontSize="sm" color="fg.muted">
              {t("settings.beta.enablePayrollHelp")}
            </Text>
          </Box>
          <Switch.Root
            checked={payrollEnabled}
            onCheckedChange={(d) => setPayroll(d.checked)}
            disabled={save.isPending}
          >
            <Switch.HiddenInput />
            <Switch.Control />
          </Switch.Root>
        </HStack>
      </Box>
    </Stack>
  );
}
