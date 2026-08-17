import { Alert, Badge, Box, Button, HStack, Spinner, Stack, Text } from "@chakra-ui/react";
import { zodResolver } from "@hookform/resolvers/zod";
import { useState } from "react";
import { useForm } from "react-hook-form";
import { useTranslation } from "react-i18next";
import { z } from "zod";

import ConfirmDialog from "../../components/ConfirmDialog";
import FormField from "../../components/FormField";
import { useServerFormErrors } from "../../lib/formErrors";
import { toast } from "../../lib/toaster";
import { useSetTunnelSettingsMutation, useTunnelSettingsQuery } from "../../queries/settings";

// The token is validated for real on the server (common.NormalizeTunnelToken);
// the schema here only stops an empty submit, because "clear the token" is a
// separate, confirmed action rather than an empty Save.
const Schema = z.object({ token: z.string().min(1) });
type FormValues = z.infer<typeof Schema>;

// SettingsTunnel (OWNER) — the Cloudflare Tunnel token that makes this shop
// reachable on a public hostname without port forwarding.
//
// Two things this panel exists to say, beyond holding the input:
//   - the token is read at BOOT, so saving one schedules a change rather than
//     making it (restartRequired), and
//   - config.yaml and the environment can outrank what is saved here, which is
//     otherwise indistinguishable from "my save did nothing".
export default function SettingsTunnel() {
  const { t } = useTranslation();
  const q = useTunnelSettingsQuery();
  const save = useSetTunnelSettingsMutation();
  const [clearing, setClearing] = useState(false);

  const form = useForm<FormValues>({ resolver: zodResolver(Schema), defaultValues: { token: "" } });
  const onServerError = useServerFormErrors(form);

  const state = q.data;

  const onSubmit = form.handleSubmit(async (v) => {
    try {
      await save.mutateAsync({ token: v.token });
      // Never leave a credential sitting in the field after it is stored.
      form.reset({ token: "" });
      toast.success(t("settings.tunnel.saved"));
    } catch (err) {
      onServerError(err);
    }
  });

  const onClear = async () => {
    try {
      await save.mutateAsync({ token: "" });
      form.reset({ token: "" });
      toast.success(t("settings.tunnel.cleared"));
    } catch {
      /* toast handled globally */
    } finally {
      setClearing(false);
    }
  };

  if (q.isLoading) {
    return (
      <Box p={8} textAlign="center">
        <Spinner />
      </Box>
    );
  }

  // One line describing what the server is actually doing.
  //
  // `active` is checked BEFORE the source, because the source only says which
  // token would win — describing a config.yaml token as the one "in use" on a
  // server that never started a tunnel would be false. And a saved-but-pending
  // token gets its own line: reading "no tunnel" directly above "saved, restart
  // to apply" is two true statements that together look like a contradiction.
  const statusKey = (() => {
    if (state?.source === "env_off") return "settings.tunnel.status.envOff";
    if (!state?.active) {
      return state?.restartRequired ? "settings.tunnel.status.pending" : "settings.tunnel.status.off";
    }
    if (state.source === "env") return "settings.tunnel.status.env";
    if (state.source === "config") return "settings.tunnel.status.config";
    return "settings.tunnel.status.active";
  })();

  return (
    <Stack gap={5} maxW="lg">
      <Stack gap={2}>
        <HStack gap={2}>
          <Text fontSize="sm" fontWeight="medium">
            {t("settings.tunnel.statusLabel")}
          </Text>
          <Badge colorPalette={state?.active ? "green" : "gray"}>
            {state?.active ? t("settings.tunnel.on") : t("settings.tunnel.offBadge")}
          </Badge>
        </HStack>
        <Text fontSize="sm" color="fg.muted">
          {t(statusKey)}
        </Text>
      </Stack>

      {state?.restartRequired && (
        <Alert.Root status="info">
          <Alert.Indicator />
          <Alert.Content>
            <Alert.Description>{t("settings.tunnel.restartRequired")}</Alert.Description>
          </Alert.Content>
        </Alert.Root>
      )}

      {state && !state.supported && (
        <Alert.Root status="warning">
          <Alert.Indicator />
          <Alert.Content>
            <Alert.Description>{t("settings.tunnel.unsupported")}</Alert.Description>
          </Alert.Content>
        </Alert.Root>
      )}

      <Box borderTopWidth="1px" pt={5}>
        <Stack gap={4}>
          <FormField
            control={form.control}
            name="token"
            label={t("settings.tunnel.token")}
            helperText={
              state?.configured
                ? t("settings.tunnel.tokenSavedHelp", { preview: state.tokenPreview })
                : t("settings.tunnel.tokenHelp")
            }
            placeholder="eyJhIjoiN2E4..."
          />
          <HStack>
            <Button colorPalette="blue" onClick={onSubmit} loading={save.isPending}>
              {t("common.save")}
            </Button>
            {state?.configured && (
              <Button variant="ghost" colorPalette="red" onClick={() => setClearing(true)}>
                {t("settings.tunnel.clear")}
              </Button>
            )}
          </HStack>
        </Stack>
      </Box>

      <ConfirmDialog
        open={clearing}
        title={t("settings.tunnel.clearTitle")}
        body={t("settings.tunnel.clearBody")}
        confirmLabel={t("settings.tunnel.clear")}
        loading={save.isPending}
        onConfirm={onClear}
        onCancel={() => setClearing(false)}
      />
    </Stack>
  );
}
