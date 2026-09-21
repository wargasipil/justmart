import {
  Box,
  Button,
  Flex,
  HStack,
  Heading,
  Input,
  Link as ChakraLink,
  Stack,
  Switch,
  Text,
} from "@chakra-ui/react";
import { Plus } from "lucide-react";
import { useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { useNavigate } from "react-router-dom";

import DatePickerField from "../../components/DatePicker";
import MoneyInput from "../../components/MoneyInput";
import ProductPickerDialog from "../../components/ProductPickerDialog";
import SupplierSelect from "../../components/SupplierSelect";
import PurchaseLinesTable from "./PurchaseLinesTable";
import type { PriceAgreement } from "../../gen/inventory_iface/v1/price_agreement_pb";
import type { Product } from "../../gen/inventory_iface/v1/product_pb";
import { formatMoney } from "../../lib/format";
import { ALL_LIMIT } from "../../lib/pagination";
import {
  type Line,
  emptyLine,
  lineNet,
  perChosenUnitGross,
  unitCostBaseOf,
} from "../../lib/purchaseLine";
import { toast } from "../../lib/toaster";
import { usePriceAgreementsQuery } from "../../queries/priceAgreements";
import {
  useCreatePurchaseOrderMutation,
  useUpdatePurchaseOrderMutation,
} from "../../queries/purchasing";

/** The whole editable state of a restock order - what an edit seeds from. */
export type PurchaseOrderSeed = {
  supplierId: string;
  invoiceNo: string;
  invoiceDate: string;
  dueAt: string;
  note: string;
  lines: Line[];
  cartDiscount: number;
  ppnEnabled: boolean;
  ppnRate: number;
};

export type PurchaseOrderFormProps = {
  /**
   * Present => edit this existing order via UpdatePurchaseOrder; absent =>
   * create a new one. The caller checks the order is still DRAFT for the UI's
   * sake; the backend refuses anything else regardless.
   */
  poId?: string;
  /** Initial values. Read ONCE, at mount - see the seeding note below. */
  seed?: PurchaseOrderSeed;
  /**
   * Edit only. UpdatePurchaseOrderRequest carries no supplier_id, so the
   * supplier is immutable once the order exists and renders as a label.
   */
  supplierName?: string;
};

/**
 * The create/edit form for a restock (purchase order).
 *
 * SEEDING IS MOUNT-TIME ONLY, on purpose: every field below is uncontrolled
 * local state, so `seed` is read through lazy useState initializers and never
 * re-applied. A late-arriving refetch must not overwrite what the user has
 * typed. The edit route therefore holds this component back until its data has
 * loaded and mounts it under a `key`, which is what makes "read once" correct
 * rather than merely convenient.
 */
export default function PurchaseOrderForm({
  poId,
  seed,
  supplierName,
}: PurchaseOrderFormProps) {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const createMut = useCreatePurchaseOrderMutation();
  const updateMut = useUpdatePurchaseOrderMutation();
  const isEdit = !!poId;

  const [supplierId, setSupplierId] = useState(seed?.supplierId ?? "");
  const [invoiceNo, setInvoiceNo] = useState(seed?.invoiceNo ?? "");
  const [invoiceDate, setInvoiceDate] = useState(seed?.invoiceDate ?? "");
  const [dueAt, setDueAt] = useState(seed?.dueAt ?? "");
  const [note, setNote] = useState(seed?.note ?? "");
  const [lines, setLines] = useState<Line[]>(seed?.lines ?? []);
  const [pickerOpen, setPickerOpen] = useState(false);
  const [cartDiscount, setCartDiscount] = useState(seed?.cartDiscount ?? 0);
  const [ppnEnabled, setPpnEnabled] = useState(seed?.ppnEnabled ?? false);
  // percent; current Indonesian default
  const [ppnRate, setPpnRate] = useState(seed?.ppnRate ?? 11);

  // Active price agreements for the chosen supplier — reference only, to warn
  // (not block) when a line's entered cost is above the agreed price. Gated on
  // a supplier so we don't fetch every supplier's agreements when none is set.
  const agreementsQ = usePriceAgreementsQuery({
    supplierId,
    includeInactive: false,
    pageSize: ALL_LIMIT,
    enabled: !!supplierId,
  });
  const agreementMap = useMemo(() => {
    const m = new Map<string, PriceAgreement>();
    for (const a of agreementsQ.rows) m.set(`${a.productId}|${a.productUnitId}`, a);
    return m;
  }, [agreementsQ.rows]);
  // Exact-unit match: an agreement only applies when the line is bought in the
  // same unit it was negotiated for.
  const agreementFor = (l: Line): PriceAgreement | undefined =>
    l.productId && l.productUnitId ? agreementMap.get(`${l.productId}|${l.productUnitId}`) : undefined;
  const isAboveAgreement = (l: Line): boolean => {
    const a = agreementFor(l);
    return a ? perChosenUnitGross(l) > Number(a.price) : false;
  };

  // Sum the NET line totals (after per-line discount) so the displayed totals
  // match what the backend computes from the same discounts.
  const subtotal = useMemo(
    () => lines.reduce((sum, l) => sum + lineNet(l), 0),
    [lines],
  );
  const discountClamped = Math.max(0, Math.min(cartDiscount, subtotal));
  const dpp = subtotal - discountClamped;
  const rateClamped = Math.max(0, Math.min(100, ppnRate || 0));
  const ppnAmount = ppnEnabled ? Math.round((dpp * rateClamped) / 100) : 0;
  const total = dpp + ppnAmount;

  const updateLine = (idx: number, patch: Partial<Line>) => {
    setLines((cur) => cur.map((l, i) => (i === idx ? { ...l, ...patch } : l)));
  };
  const removeLine = (idx: number) => setLines((cur) => cur.filter((_, i) => i !== idx));

  // One line per checked product: the picker's check set IS this table. Ids the
  // caller already had keep their line (qty / cost / discount survive a re-open);
  // new ids become fresh lines seeded from the picked Product; dropped ids go.
  // Order follows the picker so the table reads the way it was assembled.
  const applyPicked = (ids: string[], byId: Map<string, Product>) => {
    setLines((cur) => {
      const kept = new Map(cur.map((l) => [l.productId, l]));
      return ids.map((id) => {
        const existing = kept.get(id);
        if (existing) return existing;
        const p = byId.get(id);
        const units = (p?.units ?? []).filter((u) => u.purchasable && u.active);
        const base = units.find((u) => u.isBase);
        return {
          ...emptyLine(),
          productId: id,
          productName: p?.name ?? "",
          productSku: p?.sku ?? "",
          units,
          productUnitId: base?.id ?? units[0]?.id ?? "",
        };
      });
    });
  };

  const canSubmit =
    !!supplierId &&
    lines.length > 0 &&
    lines.every((l) => l.productId && l.orderedQty > 0 && l.costPerItem >= 0);

  // Identical on both paths: UpdatePurchaseOrder takes the same
  // PurchaseOrderItemInput and REPLACES the whole set of lines.
  const payload = () => ({
    invoiceNo,
    invoiceDate,
    dueAt,
    note,
    cartDiscount: BigInt(discountClamped),
    ppnEnabled,
    ppnRate: rateClamped,
    items: lines.map((l) => ({
      productId: l.productId,
      productUnitId: l.productUnitId,
      orderedQty: l.orderedQty,
      unitCostPrice: BigInt(unitCostBaseOf(l)), // GROSS per base unit (from cost/item)
      discountType: l.discountType,
      discountPerItem: l.discountPerItem,
      // PERCENT: human decimal -> basis points (12.5 -> 1250). FIXED: minor units.
      discountValue: BigInt(
        l.discountType === "PERCENT"
          ? Math.round(l.discountValue * 100)
          : l.discountValue,
      ),
    })),
  });

  const submit = async () => {
    try {
      if (poId) {
        await updateMut.mutateAsync({ id: poId, ...payload() });
        toast.success(t("common.save") + " ✓");
        navigate(`/purchasing/${poId}`);
        return;
      }
      const res = await createMut.mutateAsync({ supplierId, ...payload() });
      toast.success(t("common.create") + " ✓");
      if (res.order?.id) navigate(`/purchasing/${res.order.id}`);
      else navigate("/purchasing/all");
    } catch {
      /* toast handled globally */
    }
  };

  return (
    <Box bg="bg.subtle" borderWidth="1px" borderRadius="lg" p={5}>
      <Heading size="md" mb={4}>
        {isEdit ? t("purchasing.editPo") : t("purchasing.newPo")}
      </Heading>
      <Stack gap={4}>
        <Flex gap={3} wrap="wrap">
          <Box flex="1" minW="240px">
            <Text fontSize="sm" fontWeight="medium" color="fg.muted" mb={1}>
              {t("purchasing.supplier")} {!isEdit && "*"}
            </Text>
            {isEdit ? (
              <>
                <Text fontWeight="medium" py={2}>
                  {supplierName || "—"}
                </Text>
                <Text fontSize="xs" color="fg.muted">
                  {t("purchasing.supplierLocked")}
                </Text>
              </>
            ) : (
              <>
                <SupplierSelect
                  value={supplierId}
                  onChange={setSupplierId}
                  placeholder={t("purchasing.selectSupplier")}
                />
                <ChakraLink
                  as="button"
                  type="button"
                  fontSize="xs"
                  color="blue.500"
                  mt={1}
                  display="inline-flex"
                  alignItems="center"
                  gap={1}
                  onClick={() => navigate("/inventory/suppliers")}
                >
                  <Plus size={12} />
                  {t("purchasing.addSupplierLink")}
                </ChakraLink>
              </>
            )}
          </Box>
          <Box flex="1" minW="180px">
            <Text fontSize="sm" fontWeight="medium" color="fg.muted" mb={1}>
              {t("purchasing.invoiceNo")}
            </Text>
            <Input value={invoiceNo} onChange={(e) => setInvoiceNo(e.target.value)} />
          </Box>
        </Flex>
        <Flex gap={3} wrap="wrap">
          <Box flex="1" minW="180px">
            <Text fontSize="sm" fontWeight="medium" color="fg.muted" mb={1}>
              {t("purchasing.invoiceDate")}
            </Text>
            <DatePickerField value={invoiceDate} onChange={setInvoiceDate} />
          </Box>
          <Box flex="1" minW="180px">
            <Text fontSize="sm" fontWeight="medium" color="fg.muted" mb={1}>
              {t("purchasing.dueAt")}
            </Text>
            <DatePickerField value={dueAt} onChange={setDueAt} />
          </Box>
          <Box flex="2" minW="240px">
            <Text fontSize="sm" fontWeight="medium" color="fg.muted" mb={1}>
              {t("purchasing.note")}
            </Text>
            <Input value={note} onChange={(e) => setNote(e.target.value)} />
          </Box>
        </Flex>

        <Box>
          <HStack justify="space-between" mb={2}>
            <Heading size="sm">{t("purchasing.items")}</Heading>
            <Button size="xs" variant="outline" onClick={() => setPickerOpen(true)}>
              <Plus size={14} />
              {t("purchasing.addProduct")}
            </Button>
          </HStack>
          <PurchaseLinesTable
            lines={lines}
            onChange={updateLine}
            onRemove={removeLine}
            agreementFor={agreementFor}
            isAboveAgreement={isAboveAgreement}
            ppnRate={ppnEnabled ? rateClamped : 0}
          />
        </Box>

        <Box
          borderWidth="1px"
          borderRadius="md"
          p={4}
          maxW="420px"
          alignSelf="flex-end"
          w="full"
        >
          <Stack gap={2}>
            <HStack justify="space-between">
              <Text color="fg.muted">{t("purchasing.subtotal")}</Text>
              <Text fontFamily="mono">{formatMoney(subtotal)}</Text>
            </HStack>
            <HStack justify="space-between" align="center">
              <Text color="fg.muted">{t("purchasing.cartDiscount")}</Text>
              <MoneyInput
                size="sm"
                width="140px"
                value={cartDiscount}
                onChange={(raw) => setCartDiscount(Number(raw || 0))}
              />
            </HStack>
            <HStack justify="space-between" align="center">
              <HStack gap={2}>
                <Switch.Root
                  checked={ppnEnabled}
                  onCheckedChange={(e) => setPpnEnabled(e.checked)}
                >
                  <Switch.HiddenInput />
                  <Switch.Control />
                </Switch.Root>
                <Text color="fg.muted">{t("purchasing.ppn")}</Text>
                <Input
                  size="xs"
                  type="number"
                  width="60px"
                  value={ppnRate}
                  onChange={(e) => setPpnRate(parseInt(e.target.value, 10) || 0)}
                  disabled={!ppnEnabled}
                  min={0}
                  max={100}
                  aria-label={t("purchasing.ppnRate")}
                />
                <Text color="fg.muted">%</Text>
              </HStack>
              <Text fontFamily="mono" color={ppnEnabled ? "fg" : "fg.muted"}>
                {formatMoney(ppnAmount)}
              </Text>
            </HStack>
            <HStack justify="space-between" pt={2} borderTopWidth="1px">
              <Text fontWeight="bold">{t("purchasing.total")}</Text>
              <Text fontWeight="bold" fontFamily="mono">
                {formatMoney(total)}
              </Text>
            </HStack>
          </Stack>
        </Box>

        <HStack justify="flex-end" gap={2} pt={2}>
          <Button
            variant="ghost"
            onClick={() => navigate(poId ? `/purchasing/${poId}` : "/purchasing/all")}
          >
            {t("common.cancel")}
          </Button>
          <Button
            colorPalette="blue"
            onClick={submit}
            loading={createMut.isPending || updateMut.isPending}
            disabled={!canSubmit}
          >
            {isEdit ? t("common.save") : t("common.create")}
          </Button>
        </HStack>
      </Stack>

      <ProductPickerDialog
        open={pickerOpen}
        onClose={() => setPickerOpen(false)}
        selectedIds={lines.map((l) => l.productId)}
        onConfirm={applyPicked}
      />
    </Box>
  );
}
