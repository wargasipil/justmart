import { useRef, useState, type ChangeEvent } from "react";
import { Badge, Box, Button, HStack, Stack, Table, Text } from "@chakra-ui/react";
import { Download, FileUp, Upload } from "lucide-react";
import { useTranslation } from "react-i18next";

import EntityDialog from "../../components/EntityDialog";
import { ImportStockStatus } from "../../gen/inventory_iface/v1/batch_pb";
import { downloadCsv, parseCsv } from "../../lib/csv";
import { formatMoney } from "../../lib/format";
import { toast } from "../../lib/toaster";
import { useImportStockMutation } from "../../queries/batches";

// Canonical CSV headers + accepted aliases (matched case-insensitively).
const ALIASES: Record<string, string[]> = {
  sku: ["sku"],
  quantity: ["quantity", "qty", "jumlah"],
  unit: ["unit", "satuan", "pack"],
  cost: ["cost", "cost_price", "harga", "harga_modal", "modal"],
  batchNumber: ["batch_number", "batch", "no_batch", "batchno"],
  expiryDate: ["expiry_date", "expiry", "kedaluwarsa", "exp"],
};
const REQUIRED_FIELDS = ["sku", "quantity"] as const;
const DATE_RE = /^\d{4}-\d{2}-\d{2}$/;

// digits-only, tolerating thousands separators (IDR has no decimals).
const toDigits = (s: string) => s.replace(/[.,\s]/g, "");

type ParsedRow = {
  row: number; // 1-based for display
  sku: string;
  quantity: number; // entered qty (in `unit` if set, else base); the server applies the factor
  unit: string;
  cost: number; // per BASE unit, minor units (whole rupiah)
  batchNumber: string;
  expiryDate: string; // YYYY-MM-DD or "" (non-expiring)
  valid: boolean;
  error?: string;
};

// pick the first present header alias for a canonical field
function resolveHeader(headers: string[], field: keyof typeof ALIASES): string | null {
  for (const a of ALIASES[field]) if (headers.includes(a)) return a;
  return null;
}

export default function ImportStockDialog({
  open,
  onClose,
}: {
  open: boolean;
  onClose: () => void;
}) {
  const { t } = useTranslation();
  const inputRef = useRef<HTMLInputElement>(null);
  const importMut = useImportStockMutation();

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
          const unit = (map.unit && r[map.unit]) || "";
          const qtyRaw = (map.quantity && r[map.quantity]) || "";
          const costRaw = (map.cost && r[map.cost]) || "";
          const batchNumber = (map.batchNumber && r[map.batchNumber]) || "";
          const expiryDate = ((map.expiryDate && r[map.expiryDate]) || "").trim();
          const qtyDigits = toDigits(qtyRaw);
          const costDigits = toDigits(costRaw);
          let error = "";
          if (!sku.trim()) error = t("inventory.batches.importErrRequired");
          else if (qtyDigits === "" || !/^\d+$/.test(qtyDigits) || Number(qtyDigits) <= 0)
            error = t("inventory.batches.importErrQuantity");
          else if (costRaw.trim() !== "" && (costDigits === "" || !/^\d+$/.test(costDigits)))
            error = t("inventory.batches.importErrCost");
          else if (expiryDate !== "" && !DATE_RE.test(expiryDate))
            error = t("inventory.batches.importErrExpiry");
          return {
            row: i + 1,
            sku: sku.trim(),
            quantity: qtyDigits === "" ? 0 : Number(qtyDigits),
            unit: unit.trim(),
            cost: costDigits === "" ? 0 : Number(costDigits),
            batchNumber: batchNumber.trim(),
            expiryDate,
            valid: error === "",
            error: error || undefined,
          };
        });
        setRows(parsed);
        setParseError("");
      } catch {
        setRows([]);
        setMissingCols([]);
        setParseError(t("inventory.batches.importParseError"));
      }
    };
    reader.readAsText(file);
  };

  const onTemplate = () => {
    downloadCsv(
      "opening-stock-template.csv",
      [
        // Base-unit quantity with an explicit lot + expiry.
        { sku: "PARA500", quantity: 200, unit: "", cost: 400, batch_number: "B-2026-01", expiry_date: "2027-12-31" },
        // Entered in a larger pack: 5 box × the product's box factor = base units.
        { sku: "AMOX500", quantity: 5, unit: "box", cost: 1200, batch_number: "", expiry_date: "2027-06-30" },
        // Non-expiring goods: leave expiry_date blank.
        { sku: "SALT", quantity: 50, unit: "", cost: 100, batch_number: "", expiry_date: "" },
      ],
      [
        { key: "sku", header: "sku" },
        { key: "quantity", header: "quantity" },
        { key: "unit", header: "unit" },
        { key: "cost", header: "cost" },
        { key: "batch_number", header: "batch_number" },
        { key: "expiry_date", header: "expiry_date" },
      ],
    );
  };

  const onImport = async () => {
    const valid = rows.filter((r) => r.valid);
    if (valid.length === 0) return;
    try {
      const res = await importMut.mutateAsync({
        rows: valid.map((r) => ({
          sku: r.sku,
          quantity: BigInt(r.quantity),
          unit: r.unit,
          costPrice: BigInt(r.cost),
          batchNumber: r.batchNumber,
          expiryDate: r.expiryDate,
        })),
      });
      const errors = res.results
        .filter((x) => x.status === ImportStockStatus.ERROR)
        .map((x) => ({ sku: x.sku, message: x.message }));
      setResult({ created: res.created, skipped: res.skipped, errored: res.errored, errors });
      toast.success(
        t("inventory.batches.importSummary", {
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
          {t("inventory.batches.importTemplate")}
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
            {t("inventory.batches.importRun", { count: validCount })}
          </Button>
        </HStack>
      </HStack>
    );

  return (
    <EntityDialog open={open} onClose={close} title={t("inventory.batches.importTitle")} size="xl" footer={footer}>
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
            {t("inventory.batches.importSummary", {
              created: result.created,
              skipped: result.skipped,
              errored: result.errored,
            })}
          </Text>
          {result.errors.length > 0 && (
            <Box borderWidth="1px" borderRadius="md" p={3} maxH="240px" overflowY="auto">
              <Text fontSize="sm" fontWeight="medium" mb={2}>
                {t("inventory.batches.importResultError")}
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
              {t("inventory.batches.importChooseFile")}
            </Button>
            {fileName && (
              <Text fontSize="sm" color="fg.muted">
                {fileName}
              </Text>
            )}
          </HStack>
          <Text fontSize="xs" color="fg.muted">
            {t("inventory.batches.importHelp")}
          </Text>

          {parseError && <Text color="fg.error">{parseError}</Text>}
          {missingCols.length > 0 && (
            <Text color="fg.error">
              {t("inventory.batches.importMissingColumns", { cols: missingCols.join(", ") })}
            </Text>
          )}

          {rows.length > 0 && (
            <>
              <HStack gap={4}>
                <Text fontSize="sm">
                  {t("inventory.batches.importValidCount", { count: validCount })}
                </Text>
                {invalidCount > 0 && (
                  <Text fontSize="sm" color="fg.error">
                    {t("inventory.batches.importInvalidCount", { count: invalidCount })}
                  </Text>
                )}
              </HStack>
              <Box maxH="320px" overflowY="auto" borderWidth="1px" borderRadius="md">
                <Table.Root size="sm">
                  <Table.Header bg="bg.muted">
                    <Table.Row>
                      <Table.ColumnHeader>#</Table.ColumnHeader>
                      <Table.ColumnHeader>{t("inventory.products.sku")}</Table.ColumnHeader>
                      <Table.ColumnHeader textAlign="end">{t("inventory.batches.qty")}</Table.ColumnHeader>
                      <Table.ColumnHeader textAlign="end">{t("inventory.batches.cost")}</Table.ColumnHeader>
                      <Table.ColumnHeader>{t("inventory.batches.batchNumber")}</Table.ColumnHeader>
                      <Table.ColumnHeader>{t("inventory.batches.expiry")}</Table.ColumnHeader>
                      <Table.ColumnHeader>{t("inventory.batches.importStatus")}</Table.ColumnHeader>
                    </Table.Row>
                  </Table.Header>
                  <Table.Body>
                    {rows.map((r) => (
                      <Table.Row key={r.row}>
                        <Table.Cell color="fg.muted">{r.row}</Table.Cell>
                        <Table.Cell fontFamily="mono">{r.sku || "—"}</Table.Cell>
                        <Table.Cell textAlign="end">
                          {r.quantity}
                          {r.unit ? ` ${r.unit}` : ""}
                        </Table.Cell>
                        <Table.Cell textAlign="end">{formatMoney(r.cost)}</Table.Cell>
                        <Table.Cell>{r.batchNumber || "—"}</Table.Cell>
                        <Table.Cell>{r.expiryDate || "—"}</Table.Cell>
                        <Table.Cell>
                          {r.valid ? (
                            <Badge colorPalette="green">{t("inventory.batches.importRowValid")}</Badge>
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
