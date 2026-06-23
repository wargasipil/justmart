import { Badge, Box, Button, HStack, IconButton, Input, Spinner, Stack, Switch, Text } from "@chakra-ui/react";
import { Copy } from "lucide-react";
import { useState } from "react";
import { useTranslation } from "react-i18next";

import { toast } from "../../lib/toaster";
import {
  useApplyXenditCredentialsMutation,
  useIntegrationStatusQuery,
  useSetActiveProviderMutation,
} from "../../queries/payment";

export default function SettingsIntegrations() {
  const { t } = useTranslation();
  const status = useIntegrationStatusQuery();
  const apply = useApplyXenditCredentialsMutation();
  const setActive = useSetActiveProviderMutation();
  const [apiKey, setApiKey] = useState("");
  const [webhookToken, setWebhookToken] = useState("");

  if (status.isLoading) {
    return (
      <Box p={8} textAlign="center">
        <Spinner />
      </Box>
    );
  }

  const data = status.data;
  const xendit = data?.providers.find((p) => p.provider === "xendit");
  const configured = xendit?.configured ?? false;
  const isActive = data?.activeProvider === "xendit";
  const webhookUrl = xendit ? `${window.location.origin}${xendit.webhookUrlPath}` : "";

  const onApply = async () => {
    if (!apiKey.trim() || !webhookToken.trim()) return;
    try {
      await apply.mutateAsync({ apiKey: apiKey.trim(), webhookToken: webhookToken.trim() });
      toast.success(t("settings.integrations.saved"));
      setApiKey("");
      setWebhookToken("");
    } catch (e) {
      toast.fromError(e);
    }
  };

  const onToggleActive = async (on: boolean) => {
    try {
      await setActive.mutateAsync(on ? "xendit" : "");
    } catch (e) {
      toast.fromError(e);
    }
  };

  const copyWebhook = () => {
    void navigator.clipboard?.writeText(webhookUrl);
    toast.success(t("settings.integrations.copied"));
  };

  return (
    <Stack gap={5} maxW="2xl">
      <Box borderWidth="1px" borderRadius="lg" p={4} bg="bg.subtle">
        <HStack justify="space-between" mb={1}>
          <Text fontWeight="medium">{t("settings.integrations.xendit")}</Text>
          <Badge colorPalette={configured ? "green" : "gray"}>
            {configured ? t("settings.integrations.connected") : t("settings.integrations.notConnected")}
          </Badge>
        </HStack>
        <Text fontSize="sm" color="fg.muted">
          {t("settings.integrations.xenditDesc")}
        </Text>
        {configured && xendit?.keyMasked && (
          <Text fontSize="xs" color="fg.muted" fontFamily="mono" mt={2}>
            {t("settings.integrations.apiKey")}: {xendit.keyMasked}
          </Text>
        )}
      </Box>

      {/* Use-for-payouts toggle (only once configured) */}
      {configured && (
        <Switch.Root checked={isActive} onCheckedChange={(d) => onToggleActive(d.checked)}>
          <Switch.HiddenInput />
          <Switch.Control />
          <Switch.Label>{t("settings.integrations.useForPayouts")}</Switch.Label>
        </Switch.Root>
      )}

      {/* Webhook URL to register in the Xendit dashboard */}
      {xendit && (
        <Box>
          <Text fontSize="sm" fontWeight="medium" mb={1}>
            {t("settings.integrations.webhookUrl")}
          </Text>
          <HStack>
            <Input value={webhookUrl} readOnly fontFamily="mono" fontSize="xs" />
            <IconButton aria-label={t("settings.integrations.copy")} variant="outline" onClick={copyWebhook}>
              <Copy size={16} />
            </IconButton>
          </HStack>
          <Text fontSize="xs" color="fg.muted" mt={1}>
            {t("settings.integrations.webhookHelp")}
          </Text>
        </Box>
      )}

      {/* Credentials form (write-only) */}
      <Stack gap={3}>
        <Text fontSize="sm" fontWeight="medium">
          {t("settings.integrations.credentials")}
        </Text>
        <Box>
          <Text fontSize="xs" color="fg.muted" mb={1}>
            {t("settings.integrations.apiKey")}
          </Text>
          <Input
            type="password"
            value={apiKey}
            onChange={(e) => setApiKey(e.target.value)}
            placeholder="xnd_…"
            fontFamily="mono"
          />
        </Box>
        <Box>
          <Text fontSize="xs" color="fg.muted" mb={1}>
            {t("settings.integrations.webhookToken")}
          </Text>
          <Input
            type="password"
            value={webhookToken}
            onChange={(e) => setWebhookToken(e.target.value)}
            placeholder={t("settings.integrations.webhookTokenPlaceholder")}
            fontFamily="mono"
          />
        </Box>
        <Button
          colorPalette="blue"
          alignSelf="flex-start"
          onClick={onApply}
          loading={apply.isPending}
          disabled={!apiKey.trim() || !webhookToken.trim()}
        >
          {t("common.save")}
        </Button>
      </Stack>
    </Stack>
  );
}
