import { Badge, Box, Button, HStack, Spinner, Stack, Text, Textarea } from "@chakra-ui/react";
import { useState } from "react";
import { useTranslation } from "react-i18next";

import { BussinessType } from "../../gen/settings_iface/v1/settings_pb";
import { toast } from "../../lib/toaster";
import { useApplyLicenseMutation, useLicenseInfoQuery } from "../../queries/settings";

function modeLabel(t: (k: string) => string, type: BussinessType): string {
  switch (type) {
    case BussinessType.PHARMACY_SHOP:
      return t("settings.license.modePharmacy");
    case BussinessType.RETAIL:
      return t("settings.license.modeRetail");
    default:
      return t("settings.license.modeUnset");
  }
}

export default function SettingsLicense() {
  const { t } = useTranslation();
  const info = useLicenseInfoQuery();
  const apply = useApplyLicenseMutation();
  const [token, setToken] = useState("");

  const onApply = async () => {
    const tk = token.trim();
    if (!tk) return;
    try {
      const res = await apply.mutateAsync(tk);
      toast.success(t("settings.license.applied", { name: res.name || "—" }));
      setToken("");
    } catch (e) {
      toast.fromError(e);
    }
  };

  if (info.isLoading) {
    return (
      <Box p={8} textAlign="center">
        <Spinner />
      </Box>
    );
  }

  const data = info.data;
  const activeType = data?.type ?? BussinessType.UNSPECIFIED;

  return (
    <Stack gap={5} maxW="2xl">
      <Box borderWidth="1px" borderRadius="lg" p={4} bg="bg.subtle">
        <Text fontSize="xs" color="fg.muted" mb={1}>
          {t("settings.license.current")}
        </Text>
        <HStack gap={2}>
          <Text fontWeight="medium">
            {data?.hasLicense ? data.name || "—" : t("settings.license.none")}
          </Text>
          <Badge colorPalette={data?.hasLicense ? "blue" : "gray"}>{modeLabel(t, activeType)}</Badge>
        </HStack>
      </Box>

      <Stack gap={2}>
        <Text fontSize="sm" fontWeight="medium">
          {t("settings.license.keyLabel")}
        </Text>
        <Textarea
          value={token}
          onChange={(e) => setToken(e.target.value)}
          rows={4}
          fontFamily="mono"
          fontSize="xs"
          placeholder={t("settings.license.placeholder")}
        />
        <Text fontSize="xs" color="fg.muted">
          {t("settings.license.help")}
        </Text>
        <Button
          colorPalette="blue"
          alignSelf="flex-start"
          onClick={onApply}
          loading={apply.isPending}
          disabled={!token.trim()}
        >
          {t("settings.license.apply")}
        </Button>
      </Stack>
    </Stack>
  );
}
