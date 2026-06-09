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
import { Plus, Trash2 } from "lucide-react";
import { useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { useNavigate } from "react-router-dom";

import DatePickerField from "../../components/DatePicker";
import EnumSelect from "../../components/EnumSelect";
import MoneyInput from "../../components/MoneyInput";
import NumberInput from "../../components/NumberInput";
import SearchableSelect from "../../components/SearchableSelect";
import type { Product, ProductUnit } from "../../gen/inventory_iface/v1/product_pb";
import { formatMoney } from "../../lib/format";
import { toast } from "../../lib/toaster";
import { searchProducts } from "../../queries/products";
import { useCreatePurchaseOrderMutation } from "../../queries/purchasing";
import { searchSuppliers } from "../../queries/suppliers";

type Line = {
  productId: string;
  productUnitId: string; // chosen purchasable unit ("" => base)
  units: ProductUnit[]; // purchasable + active units of the picked product
  orderedQty: number; // in the chosen unit
  lineTotal: number; // total cost for the line (Harga modal total); unit cost is derived
};

const factorOf = (l: Line): number => {
  const u = l.units.find((x) => x.id === l.productUnitId);
  return u ? Number(u.factor) : 1;
};
const baseQtyOf = (l: Line): number => l.orderedQty * factorOf(l);
// Cost per BASE unit is derived from the line total / base qty (rounded).
const unitCostOf = (l: Line): number => {
  const base = baseQtyOf(l);
  return base > 0 ? Math.round(l.lineTotal / base) : 0;
};
const unitNameOf = (l: Line): string =>
  l.units.find((x) => x.id === l.productUnitId)?.name ?? "";

const emptyLine = (): Line => ({
  productId: "",
  productUnitId: "",
  units: [],
  orderedQty: 1,
  lineTotal: 0,
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

  const subtotal = useMemo(
    () => lines.reduce((sum, l) => sum + l.lineTotal, 0),
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
    lines.every((l) => l.productId && l.orderedQty > 0 && l.lineTotal >= 0);

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
          unitCostPrice: BigInt(unitCostOf(l)),
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
                <Table.ColumnHeader>{t("purchasing.lineTotalInput")}</Table.ColumnHeader>
                <Table.ColumnHeader>{t("purchasing.unitCostDerived")}</Table.ColumnHeader>
                <Table.ColumnHeader />
              </Table.Row>
            </Table.Header>
            <Table.Body>
              {lines.map((l, idx) => (
                <Table.Row key={idx}>
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
                      value={l.lineTotal}
                      onChange={(raw) => updateLine(idx, { lineTotal: Number(raw || 0) })}
                    />
                  </Table.Cell>
                  <Table.Cell fontFamily="mono" color="fg.muted">
                    {formatMoney(unitCostOf(l))}
                    {factorOf(l) > 1 && (
                      <Text fontSize="xs">
                        /{t("inventory.products.baseUnit").toLowerCase()}
                      </Text>
                    )}
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
              ))}
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
