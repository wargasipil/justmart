import { Button, HStack, Stack, Text } from "@chakra-ui/react";
import { zodResolver } from "@hookform/resolvers/zod";
import { useTranslation } from "react-i18next";
import { useForm } from "react-hook-form";
import { z } from "zod";

import BatchSelect from "../../components/BatchSelect";
import EntityDrawer from "../../components/EntityDrawer";
import { useResetOnOpen } from "../../lib/formReset";
import EnumSelect from "../../components/EnumSelect";
import FormField from "../../components/FormField";
import { MovementType } from "../../gen/inventory_iface/v1/stock_pb";
import { toast } from "../../lib/toaster";
import { useRecordMovementMutation } from "../../queries/stock";

// The manual ledger-entry form, split out of routes/inventory/Movements.tsx.
// Only ADJUSTMENT + WRITE_OFF are offered: every other movement type is written
// by the flow that owns it (sale, receipt, transfer, stocktake, return).

const Schema = z.object({
  batchId: z.string().min(1),
  qty: z.coerce.number().int().refine((n) => n !== 0, { params: { i18n: "validation.qtyNonZero" } }),
  type: z.coerce.number().int(),
  reason: z.string(),
});
type FormValues = z.infer<typeof Schema>;

const EMPTY_MOVEMENT: FormValues = {
  batchId: "",
  qty: 0,
  type: MovementType.ADJUSTMENT,
  reason: "",
};

export default function RecordDrawer({
  open,
  onClose,
}: {
  open: boolean;
  onClose: () => void;
}) {
  const { t } = useTranslation();
  const record = useRecordMovementMutation();
  const form = useForm<FormValues>({
    resolver: zodResolver(Schema),
    defaultValues: EMPTY_MOVEMENT,
  });
  useResetOnOpen(form, open, EMPTY_MOVEMENT);

  const submit = form.handleSubmit(async (values) => {
    try {
      await record.mutateAsync({
        batchId: values.batchId,
        qty: values.qty,
        type: values.type as MovementType,
        reason: values.reason,
      });
      toast.success(t("common.create") + " ✓");
      form.reset();
      onClose();
    } catch {
      /* toast handled globally */
    }
  });

  return (
    <EntityDrawer
      open={open}
      onClose={onClose}
      title={t("inventory.movements.recordTitle")}
      footer={
        <HStack justify="space-between">
          <Button variant="ghost" onClick={onClose}>
            {t("common.cancel")}
          </Button>
          <Button colorPalette="blue" onClick={submit} loading={record.isPending}>
            {t("inventory.movements.record")}
          </Button>
        </HStack>
      }
    >
      <form onSubmit={submit}>
        <Stack gap={4}>
          <Stack gap={1}>
            <Text fontSize="sm" fontWeight="medium" color="fg.muted">
              {t("inventory.movements.batch")} *
            </Text>
            <BatchSelect
              value={form.watch("batchId")}
              onChange={(v) => form.setValue("batchId", v)}
              // A positive ADJUSTMENT is how you correct an empty lot upward, so
              // depleted batches have to stay pickable here.
              onlyInStock={false}
            />
          </Stack>
          <Stack gap={1}>
            <Text fontSize="sm" fontWeight="medium" color="fg.muted">
              {t("inventory.movements.type")} *
            </Text>
            <EnumSelect
              value={String(form.watch("type"))}
              onChange={(v) => form.setValue("type", Number(v))}
              items={[
                { value: String(MovementType.ADJUSTMENT), label: t("inventory.movements.types.adjustment") },
                { value: String(MovementType.WRITE_OFF), label: t("inventory.movements.types.writeOff") },
              ]}
              itemToString={(o) => o.label}
              itemToValue={(o) => o.value}
            />
          </Stack>
          <FormField
            control={form.control}
            name="qty"
            label={t("inventory.movements.qty")}
            type="number"
            required
          />
          <FormField
            control={form.control}
            name="reason"
            label={t("inventory.movements.reason")}
          />
        </Stack>
      </form>
    </EntityDrawer>
  );
}
