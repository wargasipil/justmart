import { Box, Button, HStack, Stack, Switch, Text } from "@chakra-ui/react";
import { zodResolver } from "@hookform/resolvers/zod";
import { useMemo } from "react";
import { Controller, useForm } from "react-hook-form";
import { useTranslation } from "react-i18next";
import { z } from "zod";

import EntityDialog from "../../components/EntityDialog";
import FormField from "../../components/FormField";
import { ExpirySource, type Batch, type BatchExpiryChange } from "../../gen/inventory_iface/v1/batch_pb";
import { formatExpiryText } from "../../lib/expiry";
import { formatUnix } from "../../lib/format";
import { useServerFormErrors } from "../../lib/formErrors";
import { useResetOnOpen } from "../../lib/formReset";
import { toast } from "../../lib/toaster";
import { useBatchExpiryChangesQuery, useSetBatchExpiryMutation } from "../../queries/batches";
import { useUserRefs } from "../../queries/refs";

// Correct — or confirm — a received lot's expiry (SetBatchExpiry). A reason is
// required and every change is kept, because moving a date LATER is how expired
// stock goes back on sale; the lot's history is shown under the form so the
// next person sees what changed before them.
//
// Opened from the Batches page and a product's Batches tab (managers only).
// Stays mounted, driven by `batch`; the content is guarded on it.

const Schema = z
  .object({
    noExpiry: z.boolean(),
    expiryDate: z.string(),
    reason: z.string().trim().min(1),
  })
  .superRefine((v, ctx) => {
    if (!v.noExpiry && !v.expiryDate) {
      ctx.addIssue({ code: z.ZodIssueCode.custom, path: ["expiryDate"], params: { i18n: "validation.required" } });
    }
  });
type FormValues = z.infer<typeof Schema>;

export default function BatchExpiryDialog({
  batch,
  productName,
  onClose,
}: {
  batch: Batch | null;
  productName?: string;
  onClose: () => void;
}) {
  const { t } = useTranslation();
  const setExpiry = useSetBatchExpiryMutation();
  const open = !!batch;
  const form = useForm<FormValues>({
    resolver: zodResolver(Schema),
    defaultValues: { noExpiry: false, expiryDate: "", reason: "" },
  });
  useResetOnOpen(form, open, {
    noExpiry: batch?.expirySource === ExpirySource.NONE,
    expiryDate: batch && batch.expirySource !== ExpirySource.NONE ? batch.expiryDate : "",
    reason: "",
  });
  const onServerError = useServerFormErrors(form);
  const noExpiry = form.watch("noExpiry");

  const submit = form.handleSubmit(async (v) => {
    if (!batch) return;
    try {
      await setExpiry.mutateAsync({
        batchId: batch.id,
        expiryDate: v.noExpiry ? "" : v.expiryDate,
        expirySource: v.noExpiry ? ExpirySource.NONE : ExpirySource.ENTERED,
        reason: v.reason.trim(),
      });
      toast.success(t("inventory.batches.expiryDialog.saved"));
      onClose();
    } catch (err) {
      onServerError(err); // batch.reason_required / expiry_unchanged → onto their fields
    }
  });

  return (
    <EntityDialog
      open={open}
      onClose={onClose}
      title={t("inventory.batches.expiryDialog.title")}
      footer={
        <HStack justify="space-between">
          <Button variant="ghost" onClick={onClose}>
            {t("common.cancel")}
          </Button>
          <Button colorPalette="blue" onClick={submit} loading={setExpiry.isPending}>
            {t("inventory.batches.expiryDialog.save")}
          </Button>
        </HStack>
      }
    >
      {batch && (
        <form onSubmit={submit}>
          <Stack gap={4}>
            <Box>
              <Text fontWeight="medium">{productName ?? "—"}</Text>
              <Text fontSize="sm" color="fg.muted">
                {t("inventory.batches.batchNumber")}: {batch.batchNumber || "—"} ·{" "}
                {t("inventory.batches.qty")}: {batch.currentQuantity.toString()} ·{" "}
                {t("inventory.batches.expiryDialog.current")}: {expiryLabel(t, batch.expiryDate, batch.expirySource)}
              </Text>
            </Box>
            {batch.expirySource === ExpirySource.DEFAULT && (
              <Text fontSize="sm" color="fg.warning">
                {t("inventory.batches.expiryDialog.confirmHint")}
              </Text>
            )}
            <Controller
              control={form.control}
              name="noExpiry"
              render={({ field }) => (
                <Switch.Root checked={field.value} onCheckedChange={(d) => field.onChange(d.checked)}>
                  <Switch.HiddenInput />
                  <Switch.Control />
                  <Switch.Label>{t("inventory.batches.expiryDialog.noExpiry")}</Switch.Label>
                </Switch.Root>
              )}
            />
            {!noExpiry && (
              <FormField
                control={form.control}
                name="expiryDate"
                label={t("inventory.batches.expiryDialog.newDate")}
                expiry
                required
              />
            )}
            <FormField
              control={form.control}
              name="reason"
              label={t("inventory.batches.expiryDialog.reason")}
              placeholder={t("inventory.batches.expiryDialog.reasonPlaceholder")}
              required
            />
            <ExpiryHistory batchId={batch.id} />
          </Stack>
        </form>
      )}
    </EntityDialog>
  );
}

function expiryLabel(t: (k: string) => string, iso: string, source: ExpirySource): string {
  return source === ExpirySource.NONE ? t("inventory.batches.noExpiry") : formatExpiryText(iso);
}

// The lot's last few corrections, newest first. A short fixed page: this is
// context for the person editing, not an audit browser.
function ExpiryHistory({ batchId }: { batchId: string }) {
  const { t } = useTranslation();
  const q = useBatchExpiryChangesQuery(batchId, { pageSize: 5 });
  const users = useUserRefs(useMemo(() => q.rows.map((c) => c.changedBy), [q.rows]));
  return (
    <Stack gap={1} borderTopWidth="1px" pt={3}>
      <Text fontSize="sm" fontWeight="medium">
        {t("inventory.batches.expiryDialog.history")}
        {q.total > q.rows.length && ` (${q.rows.length}/${q.total})`}
      </Text>
      {q.rows.length === 0 ? (
        <Text fontSize="sm" color="fg.muted">
          {q.isLoading ? "…" : t("inventory.batches.expiryDialog.historyEmpty")}
        </Text>
      ) : (
        q.rows.map((c: BatchExpiryChange) => (
          <Box key={c.id} fontSize="sm">
            <Text>
              {t("inventory.batches.expiryDialog.historyChange", {
                from: expiryLabel(t, c.oldExpiryDate, c.oldExpirySource),
                to: expiryLabel(t, c.newExpiryDate, c.newExpirySource),
              })}{" "}
              <Text as="span" color="fg.muted">
                · {users.get(c.changedBy)?.name ?? "—"} · {formatUnix(c.changedAt)}
              </Text>
            </Text>
            <Text color="fg.muted">{c.reason}</Text>
          </Box>
        ))
      )}
    </Stack>
  );
}
