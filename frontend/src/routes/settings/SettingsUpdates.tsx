import { Badge, Box, Button, Heading, HStack, Link, Spinner, Stack, Text } from "@chakra-ui/react";
import { Download, ExternalLink, RefreshCw, Undo2 } from "lucide-react";
import { useState } from "react";
import { useTranslation } from "react-i18next";

import ConfirmDialog from "../../components/ConfirmDialog";
import { formatUnix } from "../../lib/format";
import { toast } from "../../lib/toaster";
import { useApplyUpdateMutation, useRevertUpdateMutation, useUpdateInfoQuery } from "../../queries/updates";

// SettingsUpdates (OWNER) — shows the running version vs the latest GitHub
// release. On the Windows portable build the OWNER can "Update now" (download +
// verify + stage); the launcher applies it on the next restart.
export default function SettingsUpdates() {
  const { t } = useTranslation();
  const infoQ = useUpdateInfoQuery();
  const apply = useApplyUpdateMutation();
  const revert = useRevertUpdateMutation();
  const [confirmOpen, setConfirmOpen] = useState(false);
  const [revertConfirmOpen, setRevertConfirmOpen] = useState(false);
  const [staged, setStaged] = useState<string | null>(null);
  const [revertStaged, setRevertStaged] = useState(false);

  const info = infoQ.data;

  const onApply = async () => {
    try {
      const res = await apply.mutateAsync(false);
      setStaged(res.stagedVersion);
      setConfirmOpen(false);
      toast.success(t("settings.updates.downloaded"));
    } catch {
      /* global toaster shows the error */
    }
  };

  const onRevert = async () => {
    try {
      await revert.mutateAsync();
      setRevertStaged(true);
      setRevertConfirmOpen(false);
    } catch {
      /* global toaster shows the error */
    }
  };

  if (infoQ.isLoading || !info) {
    return (
      <Box p={8} textAlign="center">
        <Spinner />
      </Box>
    );
  }

  const isDev = info.currentVersion === "dev";

  return (
    <Stack gap={5} maxW="2xl">
      <Box>
        <Heading size="sm" mb={3}>
          {t("settings.updates.title")}
        </Heading>
        <Stack gap={2} bg="bg.subtle" borderWidth="1px" borderRadius="lg" p={4}>
          <HStack justify="space-between">
            <Text fontSize="sm" color="fg.muted">
              {t("settings.updates.current")}
            </Text>
            <Text fontFamily="mono">{isDev ? t("settings.updates.devBuild") : info.currentVersion}</Text>
          </HStack>
          {info.checked && (
            <HStack justify="space-between">
              <Text fontSize="sm" color="fg.muted">
                {t("settings.updates.latest")}
              </Text>
              <HStack gap={2}>
                <Text fontFamily="mono">{info.latestVersion || "—"}</Text>
                {info.updateAvailable ? (
                  <Badge colorPalette="orange">{t("settings.updates.available")}</Badge>
                ) : (
                  <Badge colorPalette="green">{t("settings.updates.upToDate")}</Badge>
                )}
              </HStack>
            </HStack>
          )}
          {info.checked && info.publishedAt > 0n && info.updateAvailable && (
            <HStack justify="space-between">
              <Text fontSize="sm" color="fg.muted">
                {t("settings.updates.publishedAt")}
              </Text>
              <Text fontSize="sm">{formatUnix(info.publishedAt)}</Text>
            </HStack>
          )}
        </Stack>
      </Box>

      {!info.enabled && (
        <Text fontSize="sm" color="fg.muted">
          {t("settings.updates.disabled")}
        </Text>
      )}
      {info.enabled && !info.checked && (
        <Text fontSize="sm" color="fg.muted">
          {t("settings.updates.checkFailed")}
        </Text>
      )}

      {info.checked && info.updateAvailable && info.releaseNotes && (
        <Box>
          <Text fontSize="sm" fontWeight="medium" mb={1}>
            {t("settings.updates.releaseNotes")}
          </Text>
          <Box
            bg="bg.subtle"
            borderWidth="1px"
            borderRadius="md"
            p={3}
            maxH="240px"
            overflowY="auto"
            whiteSpace="pre-wrap"
            fontSize="sm"
          >
            {info.releaseNotes}
          </Box>
        </Box>
      )}

      {staged && (
        <Text fontSize="sm" color="green.fg">
          {t("settings.updates.restartHint", { version: staged })}
        </Text>
      )}

      {revertStaged && (
        <Text fontSize="sm" color="orange.fg">
          {t("settings.updates.revertStagedHint")}
        </Text>
      )}

      <HStack gap={3} wrap="wrap">
        <Button variant="outline" size="sm" onClick={() => infoQ.refetch()} loading={infoQ.isFetching}>
          <RefreshCw size={14} />
          {t("settings.updates.checkAgain")}
        </Button>

        {info.checked && info.updateAvailable && info.canSelfApply && !staged && (
          <Button colorPalette="blue" size="sm" onClick={() => setConfirmOpen(true)} loading={apply.isPending}>
            <Download size={14} />
            {t("settings.updates.updateNow")}
          </Button>
        )}

        {info.checked && info.updateAvailable && info.releaseUrl && (
          <Link href={info.releaseUrl} target="_blank" rel="noopener noreferrer" fontSize="sm" colorPalette="blue">
            <HStack gap={1}>
              <ExternalLink size={14} />
              <Text>{t("settings.updates.download")}</Text>
            </HStack>
          </Link>
        )}

        {info.canRevert && !revertStaged && (
          <Button
            variant="outline"
            colorPalette="orange"
            size="sm"
            onClick={() => setRevertConfirmOpen(true)}
            loading={revert.isPending}
          >
            <Undo2 size={14} />
            {info.backupVersion
              ? t("settings.updates.revertTo", { version: info.backupVersion })
              : t("settings.updates.revert")}
          </Button>
        )}
      </HStack>

      <ConfirmDialog
        open={confirmOpen}
        title={t("settings.updates.confirmTitle")}
        body={t("settings.updates.confirmBody", { version: info.latestVersion })}
        confirmLabel={t("settings.updates.updateNow")}
        confirmColorPalette="blue"
        loading={apply.isPending}
        onConfirm={onApply}
        onCancel={() => setConfirmOpen(false)}
      />

      <ConfirmDialog
        open={revertConfirmOpen}
        title={t("settings.updates.revertTitle")}
        body={t("settings.updates.revertBody")}
        confirmLabel={
          info.backupVersion
            ? t("settings.updates.revertTo", { version: info.backupVersion })
            : t("settings.updates.revert")
        }
        confirmColorPalette="orange"
        loading={revert.isPending}
        onConfirm={onRevert}
        onCancel={() => setRevertConfirmOpen(false)}
      />
    </Stack>
  );
}
