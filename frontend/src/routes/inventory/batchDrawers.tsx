import { useEffect, useState } from "react";
import { Button, HStack, Stack, Text } from "@chakra-ui/react";
import { zodResolver } from "@hookform/resolvers/zod";
import { useTranslation } from "react-i18next";
import { useForm } from "react-hook-form";
import { z } from "zod";

import EntityDrawer from "../../components/EntityDrawer";
import FormField from "../../components/FormField";
import ManufacturerSelect from "../../components/ManufacturerSelect";
import SearchableSelect from "../../components/SearchableSelect";
import SupplierSelect from "../../components/SupplierSelect";
import { ExpirySource } from "../../gen/inventory_iface/v1/batch_pb";
import type { Product } from "../../gen/inventory_iface/v1/product_pb";
import { formatDateOnly } from "../../lib/dateRange";
import { defaultExpiry, type ExpirySeed } from "../../lib/expiry";
import { toast } from "../../lib/toaster";
import { useCreateBatchMutation } from "../../queries/batches";
import { searchProducts } from "../../queries/products";
import { useBusinessMode } from "../../queries/settings";

// The Batches page's "add lot" drawer — hand-entered stock, outside a purchase
// order. Split out of Batches.tsx (the `<entity>Drawers.tsx` convention) when the
// expiry pre-fill pushed the page past the size threshold.

const Schema = z
  .object({
    productId: z.string().min(1),
    supplierId: z.string(),
    manufacturerId: z.string(),
    batchNumber: z.string(),
    // "Does not expire" is a product fact seeded from its setting; the date is
    // then not asked for at all.
    noExpiry: z.boolean(),
    expiryDate: z.string(),
    costPrice: z.coerce.bigint().min(0n),
    receivedAt: z.string(),
    initialQuantity: z.coerce.bigint().min(0n),
  })
  .superRefine((v, ctx) => {
    if (!v.noExpiry && !v.expiryDate) {
      ctx.addIssue({ code: z.ZodIssueCode.custom, path: ["expiryDate"], params: { i18n: "validation.required" } });
    }
  });
type FormValues = z.infer<typeof Schema>;

const EMPTY: FormValues = {
  productId: "",
  supplierId: "",
  manufacturerId: "",
  batchNumber: "",
  noExpiry: false,
  expiryDate: "",
  costPrice: 0n,
  receivedAt: "",
  initialQuantity: 0n,
};

export function CreateBatchDrawer({ open, onClose }: { open: boolean; onClose: () => void }) {
  const { t } = useTranslation();
  const create = useCreateBatchMutation();
  const { isPharmacy } = useBusinessMode();
  const form = useForm<FormValues>({ resolver: zodResolver(Schema), defaultValues: EMPTY });
  // The picked product (for its expiry setting) and what it seeded. The seed is
  // remembered so submit can tell an untouched default (DEFAULT) from a typed
  // date (ENTERED) — the server stores which, and only this side knows.
  const [product, setProduct] = useState<Product | undefined>();
  const [seed, setSeed] = useState<ExpirySeed | null>(null);

  // Re-seed the expiry — but never over a date somebody typed. A field still
  // holding the last seed (or nothing) is fair game.
  const reseed = (p: Product | undefined, receivedAt: string) => {
    const next = defaultExpiry(p, receivedAt || formatDateOnly(new Date()), isPharmacy);
    const current = form.getValues("expiryDate");
    const untouched = current === "" || current === seed?.iso;
    setSeed(next);
    form.setValue("noExpiry", next?.source === ExpirySource.NONE);
    if (untouched) form.setValue("expiryDate", next?.source === ExpirySource.DEFAULT ? next.iso : "");
  };

  const close = () => {
    form.reset(EMPTY);
    setProduct(undefined);
    setSeed(null);
    onClose();
  };

  const submit = form.handleSubmit(async ({ noExpiry, ...values }) => {
    const source = noExpiry
      ? ExpirySource.NONE
      : seed?.source === ExpirySource.DEFAULT && values.expiryDate === seed.iso
        ? ExpirySource.DEFAULT
        : ExpirySource.ENTERED;
    try {
      await create.mutateAsync({ ...values, expiryDate: noExpiry ? "" : values.expiryDate, expirySource: source });
      toast.success(t("common.create") + " ✓");
      close();
    } catch {
      /* toast handled globally */
    }
  });

  const expiryDate = form.watch("expiryDate");
  const noExpiry = form.watch("noExpiry");
  // A default counts months from the RECEIVED date, so changing that date moves
  // an untouched default with it (reseed never overwrites a typed date).
  const receivedAt = form.watch("receivedAt");
  useEffect(() => {
    if (product) reseed(product, receivedAt);
    // eslint-disable-next-line react-hooks/exhaustive-deps -- only a received-date change re-seeds
  }, [receivedAt]);

  return (
    <EntityDrawer
      open={open}
      onClose={close}
      title={t("inventory.batches.addTitle")}
      footer={
        <HStack justify="space-between">
          <Button variant="ghost" onClick={close}>
            {t("common.cancel")}
          </Button>
          <Button colorPalette="blue" onClick={submit} loading={create.isPending}>
            {t("inventory.batches.receive")}
          </Button>
        </HStack>
      }
    >
      <form onSubmit={submit}>
        <Stack gap={4}>
          <Stack gap={1}>
            <Text fontSize="sm" fontWeight="medium" color="fg.muted">
              {t("inventory.batches.product")} *
            </Text>
            <SearchableSelect
              value={form.watch("productId")}
              onChange={(v) => form.setValue("productId", v)}
              onSelectItem={(p) => {
                setProduct(p);
                reseed(p, form.getValues("receivedAt"));
              }}
              loadOptions={searchProducts}
              itemToString={(m) => `${m.sku} · ${m.name}`}
              itemToValue={(m) => m.id}
              placeholder={t("inventory.batches.selectProduct")}
            />
          </Stack>
          <Stack gap={1}>
            <Text fontSize="sm" fontWeight="medium" color="fg.muted">
              {t("inventory.batches.supplier")}
            </Text>
            <SupplierSelect
              value={form.watch("supplierId")}
              onChange={(v) => form.setValue("supplierId", v)}
              placeholder={t("inventory.batches.supplierNone")}
            />
          </Stack>
          {/* Who MADE the lot. This drawer is the one place a person is holding
              the box, so it is the one manual path that can record the fact a
              recall reads — a lot entered without it is blank forever. */}
          <Stack gap={1}>
            <Text fontSize="sm" fontWeight="medium" color="fg.muted">
              {t("inventory.products.manufacturer")}
            </Text>
            <ManufacturerSelect
              value={form.watch("manufacturerId")}
              onChange={(v) => form.setValue("manufacturerId", v)}
              placeholder={t("inventory.batches.manufacturerNone")}
            />
          </Stack>
          <FormField
            control={form.control}
            name="batchNumber"
            label={t("inventory.batches.batchNumber")}
          />
          {noExpiry ? (
            <Stack gap={0} align="start">
              <Text fontSize="sm" fontWeight="medium">
                {t("inventory.batches.expiry")}
              </Text>
              <Text color="fg.muted">{t("inventory.batches.noExpiry")}</Text>
              <Button size="2xs" variant="plain" colorPalette="blue" px={0} onClick={() => form.setValue("noExpiry", false)}>
                {t("inventory.batches.expiryDialog.newDate")}
              </Button>
            </Stack>
          ) : (
            <FormField
              control={form.control}
              name="expiryDate"
              label={t("inventory.batches.expiry")}
              expiry={{ isDefault: seed?.source === ExpirySource.DEFAULT && expiryDate === seed.iso }}
              required
            />
          )}
          <FormField
            control={form.control}
            name="receivedAt"
            label={t("inventory.batches.received")}
            type="date"
          />
          <FormField
            control={form.control}
            name="costPrice"
            label={t("inventory.batches.costPerUnit")}
            money
          />
          <FormField
            control={form.control}
            name="initialQuantity"
            label={t("inventory.batches.initialQty")}
            number
            required
          />
        </Stack>
      </form>
    </EntityDrawer>
  );
}
