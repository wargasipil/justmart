import {
  Box,
  Button,
  Flex,
  HStack,
  Heading,
  IconButton,
  Input,
  Stack,
  Table,
  Text,
} from "@chakra-ui/react";
import { Plus, Trash2 } from "lucide-react";
import { useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { useNavigate, useSearchParams } from "react-router-dom";

import DatePickerField from "../../components/DatePicker";
import EnumSelect from "../../components/EnumSelect";
import MoneyInput from "../../components/MoneyInput";
import PageHeader from "../../components/PageHeader";
import SearchableSelect from "../../components/SearchableSelect";
import type { Product, ProductUnit } from "../../gen/inventory_iface/v1/product_pb";
import { toast } from "../../lib/toaster";
import { useCreatePriceAgreementsMutation } from "../../queries/priceAgreements";
import { searchProducts } from "../../queries/products";
import { useSupplierRefs } from "../../queries/refs";
import { searchSuppliers } from "../../queries/suppliers";

type Line = {
  productId: string;
  productUnitId: string;
  units: ProductUnit[]; // active units of the picked product
  price: number;
  validFrom: string;
  validUntil: string;
  note: string;
};

const emptyLine = (): Line => ({
  productId: "",
  productUnitId: "",
  units: [],
  price: 0,
  validFrom: "",
  validUntil: "",
  note: "",
});

const dupKey = (l: Line): string => `${l.productId}|${l.productUnitId}`;

// Dedicated multi-line create page: pick ONE supplier, then add many
// product/unit/price lines. One submit → CreatePriceAgreements (all-or-nothing).
// Editing a single agreement stays the drawer (PriceAgreementDrawer).
export default function NewPriceAgreement() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const [params] = useSearchParams();
  const createMut = useCreatePriceAgreementsMutation();

  const lockedSupplierId = params.get("supplier") ?? "";
  const returnToSupplier = params.get("returnTo") === "supplier" && !!lockedSupplierId;

  const [supplierId, setSupplierId] = useState(lockedSupplierId);
  const [lines, setLines] = useState<Line[]>([emptyLine()]);

  const supplierRefs = useSupplierRefs(useMemo(() => (supplierId ? [supplierId] : []), [supplierId]));
  const supLabel = supplierId
    ? (() => {
        const s = supplierRefs.get(supplierId);
        return s ? `${s.code} · ${s.name}` : undefined;
      })()
    : undefined;

  const updateLine = (idx: number, patch: Partial<Line>) =>
    setLines((cur) => cur.map((l, i) => (i === idx ? { ...l, ...patch } : l)));
  const removeLine = (idx: number) => setLines((cur) => cur.filter((_, i) => i !== idx));
  const addLine = () => setLines((cur) => [...cur, emptyLine()]);

  const onPickProduct = (idx: number, m: Product | undefined) => {
    const units = (m?.units ?? []).filter((u) => u.active);
    const base = units.find((u) => u.isBase);
    updateLine(idx, { units, productUnitId: base?.id ?? units[0]?.id ?? "" });
  };

  // Lines that duplicate another complete line on (product, unit) — flagged
  // inline and block submit (the backend also rejects within-request dups).
  const dupIdx = useMemo(() => {
    const counts = new Map<string, number>();
    lines.forEach((l) => {
      if (l.productId && l.productUnitId) counts.set(dupKey(l), (counts.get(dupKey(l)) ?? 0) + 1);
    });
    const out = new Set<number>();
    lines.forEach((l, i) => {
      if (l.productId && l.productUnitId && (counts.get(dupKey(l)) ?? 0) > 1) out.add(i);
    });
    return out;
  }, [lines]);

  const unitNameOf = (l: Line): string => l.units.find((u) => u.id === l.productUnitId)?.name ?? "";

  const canSubmit =
    !!supplierId &&
    lines.length > 0 &&
    dupIdx.size === 0 &&
    lines.every((l) => l.productId && l.productUnitId && l.price >= 0);

  const goBack = () =>
    returnToSupplier ? navigate(`/inventory/suppliers/${supplierId}`) : navigate("/inventory/price-agreements");

  const submit = async () => {
    try {
      await createMut.mutateAsync({
        supplierId,
        items: lines.map((l) => ({
          productId: l.productId,
          productUnitId: l.productUnitId,
          price: BigInt(l.price || 0),
          validFrom: l.validFrom,
          validUntil: l.validUntil,
          note: l.note,
        })),
      });
      toast.success(t("common.create") + " ✓");
      goBack();
    } catch {
      /* toast handled globally (translateServerError) */
    }
  };

  return (
    <Box bg="bg.subtle" borderWidth="1px" borderRadius="lg" p={5}>
      <PageHeader
        breadcrumbs={[
          { label: t("nav.priceAgreements"), to: "/inventory/price-agreements" },
          { label: t("inventory.priceAgreements.newTitle") },
        ]}
        title={t("inventory.priceAgreements.newTitle")}
      />
      <Stack gap={4}>
        <Flex gap={3} wrap="wrap">
          <Box flex="1" minW="280px" maxW="420px">
            <Text fontSize="sm" fontWeight="medium" color="fg.muted" mb={1}>
              {t("inventory.priceAgreements.supplier")} *
            </Text>
            <SearchableSelect
              value={supplierId}
              onChange={setSupplierId}
              loadOptions={searchSuppliers}
              itemToString={(s) => `${s.code} · ${s.name}`}
              itemToValue={(s) => s.id}
              placeholder={t("inventory.priceAgreements.selectSupplier")}
              selectedLabel={supLabel}
              disabled={!!lockedSupplierId}
            />
          </Box>
        </Flex>

        <Box>
          <HStack justify="space-between" mb={2}>
            <Heading size="sm">{t("inventory.priceAgreements.items")}</Heading>
            <Button size="xs" variant="outline" onClick={addLine}>
              <Plus size={14} />
              {t("inventory.priceAgreements.addLine")}
            </Button>
          </HStack>
          <Box overflowX="auto">
            <Table.Root size="sm">
              <Table.Header>
                <Table.Row>
                  <Table.ColumnHeader minW="240px">{t("inventory.priceAgreements.product")}</Table.ColumnHeader>
                  <Table.ColumnHeader>{t("inventory.priceAgreements.unit")}</Table.ColumnHeader>
                  <Table.ColumnHeader>{t("inventory.priceAgreements.pricePerUnit")}</Table.ColumnHeader>
                  <Table.ColumnHeader>{t("inventory.priceAgreements.validFrom")}</Table.ColumnHeader>
                  <Table.ColumnHeader>{t("inventory.priceAgreements.validUntil")}</Table.ColumnHeader>
                  <Table.ColumnHeader>{t("inventory.priceAgreements.note")}</Table.ColumnHeader>
                  <Table.ColumnHeader />
                </Table.Row>
              </Table.Header>
              <Table.Body>
                {lines.map((l, idx) => (
                  <Table.Row key={idx} bg={dupIdx.has(idx) ? "red.subtle" : undefined}>
                    <Table.Cell>
                      <SearchableSelect
                        size="sm"
                        value={l.productId}
                        onChange={(v) => updateLine(idx, { productId: v })}
                        onSelectItem={(m) => onPickProduct(idx, m)}
                        loadOptions={searchProducts}
                        itemToString={(m) => `${m.sku} · ${m.name}`}
                        itemToValue={(m) => m.id}
                        placeholder={t("inventory.priceAgreements.selectProduct")}
                      />
                      {dupIdx.has(idx) && (
                        <Text fontSize="xs" color="red.500" mt={1}>
                          {t("inventory.priceAgreements.lineDuplicate")}
                        </Text>
                      )}
                    </Table.Cell>
                    <Table.Cell>
                      {l.units.length > 1 ? (
                        <EnumSelect
                          size="sm"
                          width="120px"
                          value={l.productUnitId}
                          onChange={(v) => updateLine(idx, { productUnitId: v })}
                          items={l.units}
                          itemToString={(u) => (Number(u.factor) > 1 ? `${u.name} ×${u.factor}` : u.name)}
                          itemToValue={(u) => u.id}
                        />
                      ) : (
                        <Text fontSize="sm" color="fg.muted">
                          {unitNameOf(l) || "—"}
                        </Text>
                      )}
                    </Table.Cell>
                    <Table.Cell>
                      <MoneyInput
                        size="sm"
                        width="140px"
                        value={l.price}
                        onChange={(raw) => updateLine(idx, { price: Number(raw || 0) })}
                      />
                    </Table.Cell>
                    <Table.Cell>
                      <DatePickerField
                        size="sm"
                        width="180px"
                        value={l.validFrom}
                        onChange={(v) => updateLine(idx, { validFrom: v })}
                      />
                    </Table.Cell>
                    <Table.Cell>
                      <DatePickerField
                        size="sm"
                        width="180px"
                        value={l.validUntil}
                        min={l.validFrom || undefined}
                        onChange={(v) => updateLine(idx, { validUntil: v })}
                      />
                    </Table.Cell>
                    <Table.Cell>
                      <Input
                        size="sm"
                        width="180px"
                        value={l.note}
                        onChange={(e) => updateLine(idx, { note: e.target.value })}
                      />
                    </Table.Cell>
                    <Table.Cell>
                      <IconButton
                        aria-label={t("inventory.priceAgreements.removeLine")}
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
        </Box>

        <HStack justify="flex-end" gap={2} pt={2}>
          <Button variant="ghost" onClick={goBack}>
            {t("common.cancel")}
          </Button>
          <Button colorPalette="blue" onClick={submit} loading={createMut.isPending} disabled={!canSubmit}>
            {t("common.create")}
          </Button>
        </HStack>
      </Stack>
    </Box>
  );
}
