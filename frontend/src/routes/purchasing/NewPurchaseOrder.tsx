import {
  Box,
  Button,
  Flex,
  HStack,
  Heading,
  IconButton,
  Input,
  Link as ChakraLink,
  Stack,
  Switch,
  Table,
  Text,
} from "@chakra-ui/react";
import { AlertTriangle, Plus, Trash2 } from "lucide-react";
import { useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { useNavigate } from "react-router-dom";

import DatePickerField from "../../components/DatePicker";
import EnumSelect from "../../components/EnumSelect";
import MoneyInput from "../../components/MoneyInput";
import NumberInput from "../../components/NumberInput";
import SearchableSelect from "../../components/SearchableSelect";
import type { PriceAgreement } from "../../gen/inventory_iface/v1/price_agreement_pb";
import type { Product, ProductUnit } from "../../gen/inventory_iface/v1/product_pb";
import { formatMoney } from "../../lib/format";
import { ALL_LIMIT } from "../../lib/pagination";
import { toast } from "../../lib/toaster";
import { usePriceAgreementsQuery } from "../../queries/priceAgreements";
import { searchProducts } from "../../queries/products";
import { useCreatePurchaseOrderMutation } from "../../queries/purchasing";
import { searchSuppliers } from "../../queries/suppliers";

type DiscountType = "FIXED" | "PERCENT";
// The 4 effective discount modes = (discountType, discountPerItem). The "_ITEM"
// modes apply the discount to each item's cost (× qty) instead of the whole line.
type DiscountMode = "FIXED" | "PERCENT" | "FIXED_ITEM" | "PERCENT_ITEM";
const DISCOUNT_MODES: DiscountMode[] = ["FIXED", "PERCENT", "FIXED_ITEM", "PERCENT_ITEM"];
const modeOf = (l: Line): DiscountMode =>
  l.discountPerItem
    ? l.discountType === "PERCENT"
      ? "PERCENT_ITEM"
      : "FIXED_ITEM"
    : l.discountType;
const modeToParts = (m: DiscountMode): { discountType: DiscountType; discountPerItem: boolean } => ({
  discountType: m === "PERCENT" || m === "PERCENT_ITEM" ? "PERCENT" : "FIXED",
  discountPerItem: m === "FIXED_ITEM" || m === "PERCENT_ITEM",
});

type Line = {
  productId: string;
  productUnitId: string; // chosen purchasable unit ("" => base)
  units: ProductUnit[]; // purchasable + active units of the picked product
  orderedQty: number; // in the chosen unit
  costPerItem: number; // GROSS cost per chosen purchasable unit (entered); line total is derived
  discountType: DiscountType;
  discountPerItem: boolean;
  discountValue: number; // FIXED: minor units; PERCENT: human decimal percent (e.g. 12.5)
};

const factorOf = (l: Line): number => {
  const u = l.units.find((x) => x.id === l.productUnitId);
  return u ? Number(u.factor) : 1;
};
const baseQtyOf = (l: Line): number => l.orderedQty * factorOf(l);
// GROSS cost per BASE unit — derived from the entered cost-per-item / factor.
// Sent to the backend as unit_cost_price; the preview below uses this same
// rounded integer so the displayed totals agree with what the server stores.
const unitCostBaseOf = (l: Line): number => Math.round(l.costPerItem / factorOf(l));
// GROSS extended line amount = base qty × per-base cost.
const grossOf = (l: Line): number => baseQtyOf(l) * unitCostBaseOf(l);
// Per-line discount amount — mirrors the backend lineNetSubtotal EXACTLY so the
// preview matches: per-item rounds each item then × qty; per-line rounds the
// whole line. PERCENT value is converted to basis points first (×100), like submit.
const lineDiscountAmount = (l: Line): number => {
  const gross = grossOf(l);
  const chosenQty = l.orderedQty;
  const isPct = l.discountType === "PERCENT";
  const val = isPct ? Math.round(l.discountValue * 100) : l.discountValue; // bp | rupiah
  let disc: number;
  if (l.discountPerItem && chosenQty > 0) {
    const perItemGross = Math.floor(gross / chosenQty); // exact (gross is a multiple of qty)
    let perItemDisc = isPct ? Math.floor((perItemGross * val + 5000) / 10000) : val;
    if (perItemDisc > perItemGross) perItemDisc = perItemGross;
    disc = perItemDisc * chosenQty;
  } else if (!isPct) {
    disc = val;
  } else {
    disc = Math.floor((gross * val + 5000) / 10000);
  }
  return Math.max(0, Math.min(disc, gross));
};
const lineNet = (l: Line): number => grossOf(l) - lineDiscountAmount(l);
// NET cost per base unit — what flows to the received batch's cost_price.
const netUnitCostOf = (l: Line): number => {
  const base = baseQtyOf(l);
  return base > 0 ? Math.round(lineNet(l) / base) : 0;
};
const unitNameOf = (l: Line): string =>
  l.units.find((x) => x.id === l.productUnitId)?.name ?? "";
// The entered cost per chosen unit — the basis for the price-agreement compare.
const perChosenUnitGross = (l: Line): number => l.costPerItem;

const emptyLine = (): Line => ({
  productId: "",
  productUnitId: "",
  units: [],
  orderedQty: 1,
  costPerItem: 0,
  discountType: "FIXED",
  discountPerItem: false,
  discountValue: 0,
});

export default function NewPurchaseOrder() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const createMut = useCreatePurchaseOrderMutation();

  const [supplierId, setSupplierId] = useState("");
  const [invoiceNo, setInvoiceNo] = useState("");
  const [invoiceDate, setInvoiceDate] = useState("");
  const [dueAt, setDueAt] = useState("");
  const [note, setNote] = useState("");
  const [lines, setLines] = useState<Line[]>([emptyLine()]);
  const [cartDiscount, setCartDiscount] = useState(0);
  const [ppnEnabled, setPpnEnabled] = useState(false);
  const [ppnRate, setPpnRate] = useState(11); // percent; current Indonesian default

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
  const addLine = () => setLines((cur) => [...cur, emptyLine()]);

  const onPickProduct = (idx: number, m: Product | undefined) => {
    const units = (m?.units ?? []).filter((u) => u.purchasable && u.active);
    const base = units.find((u) => u.isBase);
    updateLine(idx, { units, productUnitId: base?.id ?? units[0]?.id ?? "" });
  };

  const canSubmit =
    !!supplierId &&
    lines.length > 0 &&
    lines.every((l) => l.productId && l.orderedQty > 0 && l.costPerItem >= 0);

  const submit = async () => {
    try {
      const res = await createMut.mutateAsync({
        supplierId,
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
        {t("purchasing.newPo")}
      </Heading>
      <Stack gap={4}>
        <Flex gap={3} wrap="wrap">
          <Box flex="1" minW="240px">
            <Text fontSize="sm" fontWeight="medium" color="fg.muted" mb={1}>
              {t("purchasing.supplier")} *
            </Text>
            <SearchableSelect
              value={supplierId}
              onChange={setSupplierId}
              loadOptions={searchSuppliers}
              itemToString={(s) => `${s.code} · ${s.name}`}
              itemToValue={(s) => s.id}
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
            <Button size="xs" variant="outline" onClick={addLine}>
              <Plus size={14} />
              {t("purchasing.addLine")}
            </Button>
          </HStack>
          <Table.Root size="sm">
            <Table.Header>
              <Table.Row>
                <Table.ColumnHeader minW="240px">{t("purchasing.selectProduct")}</Table.ColumnHeader>
                <Table.ColumnHeader>{t("purchasing.unit")}</Table.ColumnHeader>
                <Table.ColumnHeader>{t("purchasing.qty")}</Table.ColumnHeader>
                <Table.ColumnHeader>{t("purchasing.costPerItemInput")}</Table.ColumnHeader>
                <Table.ColumnHeader>{t("purchasing.lineDiscount")}</Table.ColumnHeader>
                <Table.ColumnHeader>{t("purchasing.unitCostDerived")}</Table.ColumnHeader>
                <Table.ColumnHeader>{t("purchasing.subtotal")}</Table.ColumnHeader>
                <Table.ColumnHeader />
              </Table.Row>
            </Table.Header>
            <Table.Body>
              {lines.map((l, idx) => {
                const agreement = agreementFor(l);
                const above = isAboveAgreement(l);
                return (
                <Table.Row key={idx} bg={above ? "red.subtle" : undefined}>
                  <Table.Cell>
                    <SearchableSelect
                      size="sm"
                      value={l.productId}
                      onChange={(v) => updateLine(idx, { productId: v })}
                      onSelectItem={(m) => onPickProduct(idx, m)}
                      loadOptions={searchProducts}
                      itemToString={(m) => `${m.sku} · ${m.name}`}
                      itemToValue={(m) => m.id}
                      placeholder={t("purchasing.selectProduct")}
                    />
                    {agreement && (
                      <Stack gap={0.5} mt={1}>
                        <Text fontSize="xs" color="fg.muted">
                          {t("purchasing.agreedPrice", { price: formatMoney(Number(agreement.price)) })}
                        </Text>
                        {above && (
                          <HStack gap={1} color="red.500">
                            <AlertTriangle size={12} />
                            <Text fontSize="xs">{t("purchasing.aboveAgreement")}</Text>
                          </HStack>
                        )}
                      </Stack>
                    )}
                  </Table.Cell>
                  <Table.Cell>
                    {l.units.length > 1 ? (
                      <EnumSelect
                        size="sm"
                        width="110px"
                        value={l.productUnitId}
                        onChange={(v) => updateLine(idx, { productUnitId: v })}
                        items={l.units}
                        itemToString={(u) => u.name}
                        itemToValue={(u) => u.id}
                      />
                    ) : (
                      <Text fontSize="sm" color="fg.muted">
                        {unitNameOf(l) || "—"}
                      </Text>
                    )}
                  </Table.Cell>
                  <Table.Cell>
                    <NumberInput
                      size="sm"
                      width="80px"
                      value={l.orderedQty}
                      onChange={(raw) => updateLine(idx, { orderedQty: Number(raw || 0) })}
                    />
                  </Table.Cell>
                  <Table.Cell>
                    <MoneyInput
                      size="sm"
                      width="140px"
                      value={l.costPerItem}
                      onChange={(raw) => updateLine(idx, { costPerItem: Number(raw || 0) })}
                    />
                  </Table.Cell>
                  <Table.Cell>
                    <HStack gap={1}>
                      <EnumSelect
                        size="sm"
                        width="150px"
                        value={modeOf(l)}
                        onChange={(v) =>
                          updateLine(idx, { ...modeToParts(v as DiscountMode), discountValue: 0 })
                        }
                        items={DISCOUNT_MODES}
                        itemToString={(m) =>
                          t(
                            m === "FIXED"
                              ? "purchasing.fixed"
                              : m === "PERCENT"
                                ? "purchasing.percent"
                                : m === "FIXED_ITEM"
                                  ? "purchasing.fixedPerItem"
                                  : "purchasing.percentPerItem",
                          )
                        }
                        itemToValue={(m) => m}
                      />
                      {l.discountType === "PERCENT" ? (
                        <Input
                          size="sm"
                          type="number"
                          step="0.01"
                          min={0}
                          max={100}
                          width="74px"
                          value={l.discountValue || ""}
                          onChange={(e) =>
                            updateLine(idx, {
                              discountValue: Math.min(100, Math.max(0, Number(e.target.value) || 0)),
                            })
                          }
                          aria-label={t("purchasing.lineDiscount")}
                        />
                      ) : (
                        <MoneyInput
                          size="sm"
                          width="110px"
                          value={l.discountValue}
                          onChange={(raw) => updateLine(idx, { discountValue: Number(raw || 0) })}
                        />
                      )}
                    </HStack>
                  </Table.Cell>
                  <Table.Cell fontFamily="mono" color="fg.muted">
                    {formatMoney(netUnitCostOf(l))}
                    {factorOf(l) > 1 && (
                      <Text fontSize="xs">
                        /{t("inventory.products.baseUnit").toLowerCase()}
                      </Text>
                    )}
                  </Table.Cell>
                  <Table.Cell fontFamily="mono" fontWeight="medium">
                    {formatMoney(lineNet(l))}
                  </Table.Cell>
                  <Table.Cell>
                    <IconButton
                      aria-label="remove line"
                      size="xs"
                      variant="ghost"
                      onClick={() => removeLine(idx)}
                      disabled={lines.length === 1}
                    >
                      <Trash2 size={14} />
                    </IconButton>
                  </Table.Cell>
                </Table.Row>
                );
              })}
            </Table.Body>
          </Table.Root>
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
          <Button variant="ghost" onClick={() => navigate("/purchasing/all")}>
            {t("common.cancel")}
          </Button>
          <Button
            colorPalette="blue"
            onClick={submit}
            loading={createMut.isPending}
            disabled={!canSubmit}
          >
            {t("common.create")}
          </Button>
        </HStack>
      </Stack>
    </Box>
  );
}
