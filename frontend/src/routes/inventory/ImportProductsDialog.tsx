import { useRef, useState, type ChangeEvent } from "react";
import { Badge, Box, Button, HStack, Stack, Table, Text } from "@chakra-ui/react";
import { Download, FileUp, Upload } from "lucide-react";
import { useTranslation } from "react-i18next";

import EntityDialog from "../../components/EntityDialog";
import { ImportProductStatus } from "../../gen/inventory_iface/v1/product_pb";
import { downloadCsv, parseCsv } from "../../lib/csv";
import { formatMoney } from "../../lib/format";
import { toast } from "../../lib/toaster";
import { useImportProductsMutation } from "../../queries/products";

// Canonical CSV headers + accepted aliases (matched case-insensitively).
const ALIASES: Record<string, string[]> = {
  sku: ["sku"],
  name: ["name", "nama"],
  unit: ["unit", "satuan"],
  unitPrice: ["unit_price", "price", "unitprice", "harga"],
  prescriptionRequired: ["prescription_required", "rx", "resep", "prescription"],
  units: ["units", "packs"], // optional: extra packs "name:factor:price;name:factor:price"
};
const REQUIRED_FIELDS = ["sku", "name", "unit", "unitPrice"] as const;
const TRUTHY = new Set(["1", "true", "yes", "ya", "y", "x"]);

// digits-only, tolerating thousands separators (IDR has no decimals).
const toDigits = (s: string) => s.replace(/[.,\s]/g, "");

type ParsedUnit = { name: string; factor: number; sellPrice: number };

type ParsedRow = {
  row: number; // 1-based for display
  sku: string;
  name: string;
  unit: string;
  unitPrice: number; // minor units (whole rupiah)
  prescriptionRequired: boolean;
  units: ParsedUnit[]; // extra (non-base) packs
  valid: boolean;
  error?: string;
};

// pick the first present header alias for a canonical field
function resolveHeader(headers: string[], field: keyof typeof ALIASES): string | null {
  for (const a of ALIASES[field]) if (headers.includes(a)) return a;
  return null;
}

// parseUnits decodes the optional `units` cell ("name:factor:price;…") into the
// extra packs. Mirrors the backend syncProductUnits rules: name required, factor
// integer > 1, price integer >= 0. Returns a per-row error on the first bad entry.
function parseUnits(
  cell: string,
  t: (k: string, o?: Record<string, unknown>) => string,
): { units: ParsedUnit[]; error?: string } {
  const out: ParsedUnit[] = [];
  for (const entry of cell.split(";").map((s) => s.trim()).filter(Boolean)) {
    const parts = entry.split(":");
    if (parts.length !== 3) return { units: [], error: t("inventory.products.importErrUnitFormat") };
    const name = parts[0].trim();
    const factor = toDigits(parts[1]);
    const price = toDigits(parts[2]);
    if (!name) return { units: [], error: t("inventory.products.importErrUnitFormat") };
    if (!/^\d+$/.test(factor) || Number(factor) <= 1)
      return { units: [], error: t("inventory.products.importErrUnitFactor", { name }) };
    if (price === "" || !/^\d+$/.test(price))
      return { units: [], error: t("inventory.products.importErrUnitPrice", { name }) };
    out.push({ name, factor: Number(factor), sellPrice: Number(price) });
  }
  return { units: out };
}

export default function ImportProductsDialog({
  open,
  onClose,
}: {
  open: boolean;
  onClose: () => void;
}) {
  const { t } = useTranslation();
  const inputRef = useRef<HTMLInputElement>(null);
  const importMut = useImportProductsMutation();

  const [fileName, setFileName] = useState("");
  const [rows, setRows] = useState<ParsedRow[]>([]);
  const [missingCols, setMissingCols] = useState<string[]>([]);
  const [parseError, setParseError] = useState("");
  const [result, setResult] = useState<{
    created: number;
    skipped: number;
    errored: number;
    errors: { sku: string; message: string }[];
  } | null>(null);

  const reset = () => {
    setFileName("");
    setRows([]);
    setMissingCols([]);
    setParseError("");
    setResult(null);
    if (inputRef.current) inputRef.current.value = "";
  };

  const close = () => {
    reset();
    onClose();
  };

  const validCount = rows.filter((r) => r.valid).length;
  const invalidCount = rows.length - validCount;

  const onPickFile = () => inputRef.current?.click();

  const onFileChange = (e: ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;
    setFileName(file.name);
    setResult(null);
    const reader = new FileReader();
    reader.onload = (ev) => {
      try {
        const { headers, rows: raw } = parseCsv(String(ev.target?.result ?? ""));
        const map = Object.fromEntries(
          (Object.keys(ALIASES) as (keyof typeof ALIASES)[]).map((f) => [f, resolveHeader(headers, f)]),
        ) as Record<keyof typeof ALIASES, string | null>;

        const missing = REQUIRED_FIELDS.filter((f) => !map[f]);
        if (missing.length > 0) {
          setMissingCols(missing.map((f) => ALIASES[f][0]));
          setRows([]);
          setParseError("");
          return;
        }
        setMissingCols([]);

        const parsed: ParsedRow[] = raw.map((r, i) => {
          const sku = (map.sku && r[map.sku]) || "";
          const name = (map.name && r[map.name]) || "";
          const unit = (map.unit && r[map.unit]) || "";
          const priceRaw = (map.unitPrice && r[map.unitPrice]) || "";
          const rxRaw = (map.prescriptionRequired && r[map.prescriptionRequired]) || "";
          const unitsRaw = (map.units && r[map.units]) || "";
          const digits = toDigits(priceRaw);
          const { units, error: unitsErr } = parseUnits(unitsRaw, t);
          let error = "";
          if (!sku || !name || !unit) error = t("inventory.products.importErrRequired");
          else if (digits === "" || !/^\d+$/.test(digits)) error = t("inventory.products.importErrPrice");
          else if (unitsErr) error = unitsErr;
          return {
            row: i + 1,
            sku,
            name,
            unit,
            unitPrice: digits === "" ? 0 : Number(digits),
            prescriptionRequired: TRUTHY.has(rxRaw.toLowerCase()),
            units,
            valid: error === "",
            error: error || undefined,
          };
        });
        setRows(parsed);
        setParseError("");
      } catch {
        setRows([]);
        setMissingCols([]);
        setParseError(t("inventory.products.importParseError"));
      }
    };
    reader.readAsText(file);
  };

  const onTemplate = () => {
    downloadCsv(
      "products-template.csv",
      [
        // Multi-unit example: base "tablet" + two larger packs (name:factor:price).
        {
          sku: "AMOX500",
          name: "Amoxicillin 500mg",
          unit: "tablet",
          unit_price: 1500,
          prescription_required: "yes",
          units: "box:100:140000;strip:10:15000",
        },
        // Single larger pack.
        {
          sku: "ORS-SACHET",
          name: "Oralit",
          unit: "sachet",
          unit_price: 2000,
          prescription_required: "no",
          units: "box:24:45000",
        },
        // Base unit only — leave `units` empty.
        {
          sku: "PARA500",
          name: "Paracetamol 500mg",
          unit: "tablet",
          unit_price: 500,
          prescription_required: "no",
          units: "",
        },
      ],
      [
        { key: "sku", header: "sku", text: true },
        { key: "name", header: "name" },
        { key: "unit", header: "unit" },
        { key: "unit_price", header: "unit_price" },
        { key: "prescription_required", header: "prescription_required" },
        { key: "units", header: "units" },
      ],
    );
  };

  const onImport = async () => {
    const valid = rows.filter((r) => r.valid);
    if (valid.length === 0) return;
    try {
      const res = await importMut.mutateAsync({
        products: valid.map((r) => ({
          sku: r.sku,
          name: r.name,
          unit: r.unit,
          unitPrice: BigInt(r.unitPrice),
          prescriptionRequired: r.prescriptionRequired,
          units: r.units.map((u) => ({
            name: u.name,
            factor: BigInt(u.factor),
            sellPrice: BigInt(u.sellPrice),
            sellable: true,
            purchasable: true,
          })),
        })),
      });
      const errors = res.results
        .filter((x) => x.status === ImportProductStatus.ERROR)
        .map((x) => ({ sku: x.sku, message: x.message }));
      setResult({ created: res.created, skipped: res.skipped, errored: res.errored, errors });
      toast.success(
        t("inventory.products.importSummary", {
          created: res.created,
          skipped: res.skipped,
          errored: res.errored,
        }),
      );
    } catch (err) {
      toast.fromError(err);
    }
  };

  const footer =
    result != null ? (
      <HStack justify="flex-end">
        <Button colorPalette="blue" onClick={close}>
          {t("common.close")}
        </Button>
      </HStack>
    ) : (
      <HStack justify="space-between">
        <Button variant="ghost" onClick={onTemplate}>
          <Download size={16} />
          {t("inventory.products.importTemplate")}
        </Button>
        <HStack>
          <Button variant="ghost" onClick={close}>
            {t("common.cancel")}
          </Button>
          <Button
            colorPalette="blue"
            onClick={onImport}
            disabled={validCount === 0}
            loading={importMut.isPending}
          >
            <Upload size={16} />
            {t("inventory.products.importRun", { count: validCount })}
          </Button>
        </HStack>
      </HStack>
    );

  return (
    <EntityDialog open={open} onClose={close} title={t("inventory.products.importTitle")} size="xl" footer={footer}>
      {/* hidden native file input (file pickers are OS chrome by necessity) */}
      <input
        ref={inputRef}
        type="file"
        accept=".csv,text/csv"
        style={{ display: "none" }}
        onChange={onFileChange}
      />

      {result != null ? (
        <Stack gap={3}>
          <Text>
            {t("inventory.products.importSummary", {
              created: result.created,
              skipped: result.skipped,
              errored: result.errored,
            })}
          </Text>
          {result.errors.length > 0 && (
            <Box borderWidth="1px" borderRadius="md" p={3} maxH="240px" overflowY="auto">
              <Text fontSize="sm" fontWeight="medium" mb={2}>
                {t("inventory.products.importResultError")}
              </Text>
              <Stack gap={1}>
                {result.errors.map((e, i) => (
                  <Text key={i} fontSize="sm" color="fg.muted" fontFamily="mono">
                    {e.sku || "—"}: {e.message}
                  </Text>
                ))}
              </Stack>
            </Box>
          )}
        </Stack>
      ) : (
        <Stack gap={3}>
          <HStack>
            <Button variant="outline" onClick={onPickFile}>
              <FileUp size={16} />
              {t("inventory.products.importChooseFile")}
            </Button>
            {fileName && (
              <Text fontSize="sm" color="fg.muted">
                {fileName}
              </Text>
            )}
          </HStack>
          <Text fontSize="xs" color="fg.muted">
            {t("inventory.products.importHelp")}
          </Text>

          {parseError && <Text color="fg.error">{parseError}</Text>}
          {missingCols.length > 0 && (
            <Text color="fg.error">
              {t("inventory.products.importMissingColumns", { cols: missingCols.join(", ") })}
            </Text>
          )}

          {rows.length > 0 && (
            <>
              <HStack gap={4}>
                <Text fontSize="sm">
                  {t("inventory.products.importValidCount", { count: validCount })}
                </Text>
                {invalidCount > 0 && (
                  <Text fontSize="sm" color="fg.error">
                    {t("inventory.products.importInvalidCount", { count: invalidCount })}
                  </Text>
                )}
              </HStack>
              <Box maxH="320px" overflowY="auto" borderWidth="1px" borderRadius="md">
                <Table.Root size="sm">
                  <Table.Header bg="bg.muted">
                    <Table.Row>
                      <Table.ColumnHeader>#</Table.ColumnHeader>
                      <Table.ColumnHeader>{t("inventory.products.sku")}</Table.ColumnHeader>
                      <Table.ColumnHeader>{t("inventory.products.name")}</Table.ColumnHeader>
                      <Table.ColumnHeader>{t("inventory.products.unit")}</Table.ColumnHeader>
                      <Table.ColumnHeader textAlign="end">{t("inventory.products.unitPrice")}</Table.ColumnHeader>
                      <Table.ColumnHeader>{t("inventory.products.importUnitsHeader")}</Table.ColumnHeader>
                      <Table.ColumnHeader>{t("inventory.products.importStatus")}</Table.ColumnHeader>
                    </Table.Row>
                  </Table.Header>
                  <Table.Body>
                    {rows.map((r) => (
                      <Table.Row key={r.row}>
                        <Table.Cell color="fg.muted">{r.row}</Table.Cell>
                        <Table.Cell fontFamily="mono">{r.sku || "—"}</Table.Cell>
                        <Table.Cell>{r.name || "—"}</Table.Cell>
                        <Table.Cell>{r.unit || "—"}</Table.Cell>
                        <Table.Cell textAlign="end">{formatMoney(r.unitPrice)}</Table.Cell>
                        <Table.Cell color="fg.muted">
                          {r.units.length > 0
                            ? r.units.map((u) => `${u.name}×${u.factor}`).join(" · ")
                            : "—"}
                        </Table.Cell>
                        <Table.Cell>
                          {r.valid ? (
                            <Badge colorPalette="green">{t("inventory.products.importRowValid")}</Badge>
                          ) : (
                            <Badge colorPalette="red" title={r.error}>
                              {r.error}
                            </Badge>
                          )}
                        </Table.Cell>
                      </Table.Row>
                    ))}
                  </Table.Body>
                </Table.Root>
              </Box>
            </>
          )}
        </Stack>
      )}
    </EntityDialog>
  );
}
