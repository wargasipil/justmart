import { useState } from "react";
import {
  Badge,
  Box,
  Breadcrumb,
  Button,
  Code,
  HStack,
  SimpleGrid,
  Stack,
  Table,
  Text,
} from "@chakra-ui/react";
import { ChevronRight } from "lucide-react";
import { zodResolver } from "@hookform/resolvers/zod";
import { useForm } from "react-hook-form";
import { useTranslation } from "react-i18next";
import { z } from "zod";

import BackButton from "../../components/BackButton";
import Barcode from "../../components/Barcode";
import BatchSelect from "../../components/BatchSelect";
import ChartCard from "../../components/ChartCard";
import ColumnsPopover from "../../components/ColumnsPopover";
import ConfirmDialog from "../../components/ConfirmDialog";
import DashboardTile from "../../components/DashboardTile";
import DatePickerField from "../../components/DatePicker";
import DateRangeFilter from "../../components/DateRangeFilter";
import DiscountField, { type DiscountType } from "../../components/DiscountField";
import EntityDialog from "../../components/EntityDialog";
import EntityDrawer from "../../components/EntityDrawer";
import EnumSelect from "../../components/EnumSelect";
import ExpiryBadge from "../../components/ExpiryBadge";
import ExportButton from "../../components/ExportButton";
import FormField from "../../components/FormField";
import MetricTable from "../../components/MetricTable";
import MoneyInput from "../../components/MoneyInput";
import NumberInput from "../../components/NumberInput";
import PageHeader from "../../components/PageHeader";
import Pagination from "../../components/Pagination";
import RouteTabs from "../../components/RouteTabs";
import SearchableSelect from "../../components/SearchableSelect";
import StockUnitPopover from "../../components/StockUnitPopover";
import SummaryTile from "../../components/SummaryTile";
import SupplierSelect from "../../components/SupplierSelect";
import TableScroll from "../../components/TableScroll";
import TrendChart from "../../components/TrendChart";
import UserAvatar from "../../components/UserAvatar";
import ProductImage from "../../components/ProductImage";
import ProductPickerDialog from "../../components/ProductPickerDialog";
import WarehouseSelect from "../../components/WarehouseSelect";
import {
  MetricOrder,
  MetricStock,
  MetricType,
  Sort,
} from "../../gen/analytics_iface/v1/analytics_pb";
import { Warehouse } from "../../gen/warehouse_iface/v1/warehouse_pb";
import {
  formatAbsolute,
  rangeBounds,
  resolveRange,
  type DateRange,
} from "../../lib/dateRange";
import { usePageState } from "../../lib/pagination";
import type { StockUnitGroup, StockUnitsByBase } from "../../lib/stockUnit";
import { toast } from "../../lib/toaster";

// Live demos for the dev-only component gallery (/components).
//
// Each export is a self-contained component that renders ONE shared component
// with local state — no server calls, no query hooks. Fixture text inside a demo
// (sample product names, codes, dates) is illustrative sample data, the same
// exemption SKUs/IDs get from the i18n rule; the gallery's own chrome is
// translated (see `dev.components.*` in src/locales).

// --- small helpers shared by the demos -------------------------------------

// Emitted-value readout: shows what a controlled component just handed back, so
// the preview doubles as a contract check (e.g. MoneyInput emits "200000").
function Emitted({ children }: { children: string }) {
  return (
    <HStack gap={2}>
      <Text fontSize="xs" color="fg.muted">
        onChange →
      </Text>
      <Code size="sm">{children === "" ? '""' : children}</Code>
    </HStack>
  );
}

function isoInDays(days: number): string {
  const d = new Date();
  d.setDate(d.getDate() + days);
  return d.toISOString().slice(0, 10);
}

// --- Layout & page chrome ---------------------------------------------------

export function PageHeaderDemo() {
  return (
    <Box borderWidth="1px" borderRadius="md" p={4}>
      <PageHeader
        title="Batch"
        titleBadge={<Badge colorPalette="green">Aktif</Badge>}
        description="Every lot with stock in the active warehouse."
        actions={
          <Button size="sm" colorPalette="blue">
            Add
          </Button>
        }
      />
      <Text fontSize="sm" color="fg.muted">
        Page body starts here.
      </Text>
    </Box>
  );
}

// The real <Breadcrumbs/> reads the live URL (and the business mode, for the
// catalog noun), so the gallery shows a static stand-in of what it renders in
// the TopBar rather than mounting the router-driven component.
export function BreadcrumbsDemo() {
  return (
    <Stack gap={3}>
      <Box borderWidth="1px" borderRadius="md" px={4} h="56px" display="flex" alignItems="center">
        <Breadcrumb.Root size="sm">
          <Breadcrumb.List flexWrap="nowrap" whiteSpace="nowrap">
            {/* A nav-group ancestor: names a sidebar section with no page of its own. */}
            <Breadcrumb.Item>
              <Text color="fg.muted">Inventaris</Text>
            </Breadcrumb.Item>
            <Breadcrumb.Separator>
              <ChevronRight size={14} />
            </Breadcrumb.Separator>
            <Breadcrumb.Item>
              <Breadcrumb.Link href="#" color="fg.muted">
                Stok opname
              </Breadcrumb.Link>
            </Breadcrumb.Item>
            <Breadcrumb.Separator>
              <ChevronRight size={14} />
            </Breadcrumb.Separator>
            {/* Leaf = where you are. On a :id route this comes from useCrumbLabel(). */}
            <Breadcrumb.Item>
              <Breadcrumb.CurrentLink>Opname Juli 2026</Breadcrumb.CurrentLink>
            </Breadcrumb.Item>
          </Breadcrumb.List>
        </Breadcrumb.Root>
      </Box>
      <Text fontSize="sm" color="fg.muted">
        Mounted once in the TopBar — a page never renders it.
      </Text>
    </Stack>
  );
}

export function RouteTabsDemo() {
  const { t } = useTranslation();
  return (
    <Stack gap={2}>
      <RouteTabs
        items={[
          { value: "layout", to: "/components/layout", label: t("dev.components.groups.layout") },
          { value: "forms", to: "/components/forms", label: t("dev.components.groups.forms") },
          { value: "data", to: "/components/data", label: t("dev.components.groups.data") },
        ]}
      />
      <Text fontSize="xs" color="fg.muted">
        {t("dev.components.demo.routeTabsHint")}
      </Text>
    </Stack>
  );
}

export function BackButtonDemo() {
  return <BackButton to="/components/layout" />;
}

export function DashboardTileDemo() {
  return (
    <SimpleGrid columns={{ base: 1, md: 3 }} gap={4}>
      <DashboardTile label="Revenue today" value="Rp 4.250.000" hint="12 sales" />
      <DashboardTile label="Low stock" value="7" tone="warning" to="/components/layout" />
      <DashboardTile label="Expired lots" value="2" tone="danger" />
    </SimpleGrid>
  );
}

export function SummaryTileDemo() {
  return (
    <SimpleGrid columns={{ base: 1, sm: 2, lg: 4 }} gap={3}>
      <SummaryTile label="Ready stock" value="12.480" />
      <SummaryTile label="Ready value" value="Rp 184.320.000" />
      <SummaryTile label="Ongoing stock" value="3.200" />
      <SummaryTile label="Ongoing value" value="Rp 41.900.000" />
    </SimpleGrid>
  );
}

// --- Forms & inputs --------------------------------------------------------

const FormDemoSchema = z.object({
  name: z.string().min(1),
  password: z.string().min(8),
  // money / number fields emit a raw digit string, so a string field + a
  // z.coerce.* at the call site is the shape real forms use.
  price: z.string(),
  qty: z.string(),
  expiry: z.string(),
  sku: z.string(),
});
type FormDemoValues = z.infer<typeof FormDemoSchema>;

export function FormFieldDemo() {
  const form = useForm<FormDemoValues>({
    resolver: zodResolver(FormDemoSchema),
    defaultValues: { name: "", password: "", price: "", qty: "", expiry: "", sku: "SKU-0001" },
  });
  return (
    <Stack gap={4} maxW="480px">
      <FormField control={form.control} name="name" label="Name" required placeholder="Paracetamol 500mg" />
      <FormField
        control={form.control}
        name="password"
        label="Password"
        type="password"
        passwordToggle
        helperText="min 8 characters"
      />
      <FormField control={form.control} name="price" label="Unit price" money />
      <FormField control={form.control} name="qty" label="Quantity" number />
      <FormField control={form.control} name="expiry" label="Expiry" type="date" />
      <FormField control={form.control} name="sku" label="SKU" disabled helperText="immutable on edit" />
      <HStack>
        <Button
          size="sm"
          colorPalette="blue"
          onClick={form.handleSubmit((v) => toast.success("Submit", JSON.stringify(v)))}
        >
          Submit
        </Button>
        <Button size="sm" variant="ghost" onClick={() => form.reset()}>
          Reset
        </Button>
      </HStack>
    </Stack>
  );
}

export function MoneyInputDemo() {
  const [raw, setRaw] = useState("200000");
  return (
    <Stack gap={2} maxW="240px">
      <MoneyInput value={raw} onChange={setRaw} aria-label="price" />
      <Emitted>{raw}</Emitted>
    </Stack>
  );
}

export function NumberInputDemo() {
  const [raw, setRaw] = useState("12");
  return (
    <Stack gap={2} maxW="240px">
      <NumberInput value={raw} onChange={setRaw} max={999} aria-label="quantity" />
      <Emitted>{raw}</Emitted>
    </Stack>
  );
}

export function DatePickerDemo() {
  const [value, setValue] = useState(isoInDays(0));
  return (
    <Stack gap={2} maxW="260px">
      <DatePickerField value={value} onChange={setValue} min={isoInDays(-30)} max={isoInDays(365)} />
      <Emitted>{value}</Emitted>
    </Stack>
  );
}

export function DiscountFieldDemo() {
  const [type, setType] = useState<DiscountType>("FIXED");
  const [value, setValue] = useState(5000);
  return (
    <Stack gap={2}>
      <DiscountField
        type={type}
        value={value}
        onChange={(nextType, nextValue) => {
          setType(nextType);
          setValue(nextValue);
        }}
      />
      <Emitted>{`${type} / ${value}`}</Emitted>
    </Stack>
  );
}

// --- Selects & pickers ----------------------------------------------------

type StatusOption = { value: string; label: string };
const STATUS_OPTIONS: StatusOption[] = [
  { value: "DRAFT", label: "Draft" },
  { value: "SENT", label: "Sent" },
  { value: "RECEIVED", label: "Received" },
  { value: "VOIDED", label: "Voided" },
];

export function EnumSelectDemo() {
  const [value, setValue] = useState("SENT");
  return (
    <Stack gap={2} maxW="240px">
      <EnumSelect
        value={value}
        onChange={setValue}
        items={STATUS_OPTIONS}
        itemToString={(o) => o.label}
        itemToValue={(o) => o.value}
        placeholder="Status"
      />
      <Emitted>{value}</Emitted>
    </Stack>
  );
}

type FakeProduct = { id: string; name: string };
const FAKE_PRODUCTS: FakeProduct[] = [
  { id: "p1", name: "Paracetamol 500mg" },
  { id: "p2", name: "Amoxicillin 500mg" },
  { id: "p3", name: "Vitamin C 1000mg" },
  { id: "p4", name: "Ibuprofen 400mg" },
  { id: "p5", name: "Cetirizine 10mg" },
];

// Stands in for a `Search<Domain>` RPC: same async contract, 200ms of fake
// latency so the debounce + loading state are visible.
function fakeSearch(query: string): Promise<FakeProduct[]> {
  const needle = query.trim().toLowerCase();
  const rows = needle
    ? FAKE_PRODUCTS.filter((p) => p.name.toLowerCase().includes(needle))
    : FAKE_PRODUCTS;
  return new Promise((resolve) => setTimeout(() => resolve(rows), 200));
}

// renderItem fixture — mirrors the shape CashierFilterSelect renders. `version: 0`
// on the avatar means "no picture", which disables the fetch entirely, so this
// demo stays server-free like every other one in this file.
type FakePerson = { id: string; name: string; role: string };
const FAKE_PEOPLE: FakePerson[] = [
  { id: "u1", name: "Budi Santoso", role: "Cashier" },
  { id: "u2", name: "Siti Rahayu", role: "Admin" },
  { id: "u3", name: "Ahmad Wijaya", role: "Cashier" },
];

function fakePeopleSearch(query: string): Promise<FakePerson[]> {
  const needle = query.trim().toLowerCase();
  const rows = needle
    ? FAKE_PEOPLE.filter((p) => p.name.toLowerCase().includes(needle))
    : FAKE_PEOPLE;
  return new Promise((resolve) => setTimeout(() => resolve(rows), 200));
}

export function SearchableSelectDemo() {
  const { t } = useTranslation();
  const [asyncValue, setAsyncValue] = useState("");
  const [syncValue, setSyncValue] = useState("");
  const [personValue, setPersonValue] = useState("");
  return (
    <Stack gap={4} maxW="360px">
      <Stack gap={2}>
        <Text fontSize="xs" color="fg.muted">
          {t("dev.components.demo.asyncMode")}
        </Text>
        <SearchableSelect
          value={asyncValue}
          onChange={setAsyncValue}
          loadOptions={fakeSearch}
          itemToString={(p: FakeProduct) => p.name}
          itemToValue={(p: FakeProduct) => p.id}
          placeholder="Search product…"
        />
        <Emitted>{asyncValue}</Emitted>
      </Stack>
      <Stack gap={2}>
        <Text fontSize="xs" color="fg.muted">
          {t("dev.components.demo.syncMode")}
        </Text>
        <SearchableSelect
          value={syncValue}
          onChange={setSyncValue}
          items={STATUS_OPTIONS}
          itemToString={(o) => o.label}
          itemToValue={(o) => o.value}
          placeholder="Status"
        />
        <Emitted>{syncValue}</Emitted>
      </Stack>
      <Stack gap={2}>
        <Text fontSize="xs" color="fg.muted">
          {t("dev.components.demo.renderItemMode")}
        </Text>
        <SearchableSelect
          value={personValue}
          onChange={setPersonValue}
          loadOptions={fakePeopleSearch}
          itemToString={(p: FakePerson) => p.name}
          itemToValue={(p: FakePerson) => p.id}
          renderItem={(p: FakePerson) => (
            <HStack gap={3} flex="1" minW={0}>
              <UserAvatar userId={p.id} name={p.name} version={0} />
              <Stack gap={0} flex="1" minW={0}>
                <Text fontWeight="medium" truncate>
                  {p.name}
                </Text>
                <Text fontSize="xs" color="fg.muted" truncate>
                  {p.role}
                </Text>
              </Stack>
            </HStack>
          )}
          placeholder="Search person…"
        />
        <Emitted>{personValue}</Emitted>
      </Stack>
    </Stack>
  );
}

// NOTE: like ProductPickerDialogDemo, this one talks to the server — the baked-in
// searchSuppliers IS the component, so a fixture list would demo the opposite of
// the thing. Idle until the popover opens (SearchableSelect fetches on first open,
// and the label resolve is disabled while nothing is selected).
export function SupplierSelectDemo() {
  const [value, setValue] = useState("");
  return (
    <Stack gap={2} maxW="280px">
      <SupplierSelect value={value} onChange={setValue} />
      <Emitted>{value}</Emitted>
    </Stack>
  );
}

const FAKE_WAREHOUSES = [
  new Warehouse({ id: "w1", code: "MAIN", name: "Gudang Utama" }),
  new Warehouse({ id: "w2", code: "CAB-01", name: "Cabang Kemang" }),
  new Warehouse({ id: "w3", code: "CAB-02", name: "Cabang Bintaro" }),
];

export function WarehouseSelectDemo() {
  const [value, setValue] = useState("w1");
  return (
    <Stack gap={2} maxW="280px">
      <WarehouseSelect warehouses={FAKE_WAREHOUSES} value={value} onChange={setValue} />
      <Emitted>{value}</Emitted>
    </Stack>
  );
}

// NOTE: like ProductPickerDialog below, this one talks to the server — BatchSelect
// IS a backend batch search, so a fixture list would demo the opposite of the
// thing. It stays idle until the dialog is opened, so browsing the gallery costs
// nothing. Left on the default onlyInStock so the demo shows the real default.
export function BatchSelectDemo() {
  const [value, setValue] = useState("");
  return (
    <Stack gap={2} maxW="320px">
      <BatchSelect value={value} onChange={setValue} clearable />
      <Emitted>{value}</Emitted>
    </Stack>
  );
}

// NOTE: the one demo in this file that talks to the server. ProductPickerDialog
// IS server-paginated search — a fixture list would demo the opposite of the
// thing. It stays idle until opened (`enabled: open`), so browsing the gallery
// costs nothing.
export function ProductPickerDialogDemo() {
  const [open, setOpen] = useState(false);
  const [ids, setIds] = useState<string[]>([]);
  const [names, setNames] = useState<string[]>([]);
  return (
    <Stack gap={2} maxW="360px">
      <Button size="sm" variant="outline" onClick={() => setOpen(true)}>
        Add products
      </Button>
      <Emitted>{names.length ? names.join(", ") : ""}</Emitted>
      <ProductPickerDialog
        open={open}
        onClose={() => setOpen(false)}
        selectedIds={ids}
        onConfirm={(picked, byId) => {
          setIds(picked);
          setNames(picked.map((id) => byId.get(id)?.name ?? id));
        }}
      />
    </Stack>
  );
}

// --- Overlays -------------------------------------------------------------

export function EntityDrawerDemo() {
  const [open, setOpen] = useState(false);
  return (
    <>
      <Button size="sm" colorPalette="blue" onClick={() => setOpen(true)}>
        Open drawer
      </Button>
      <EntityDrawer
        open={open}
        onClose={() => setOpen(false)}
        title="New supplier"
        footer={
          <HStack justify="flex-end" gap={2}>
            <Button variant="ghost" onClick={() => setOpen(false)}>
              Cancel
            </Button>
            <Button colorPalette="blue" onClick={() => setOpen(false)}>
              Save
            </Button>
          </HStack>
        }
      >
        <Text fontSize="sm" color="fg.muted">
          Form fields go here — one FormField per row.
        </Text>
      </EntityDrawer>
    </>
  );
}

export function EntityDialogDemo() {
  const [open, setOpen] = useState(false);
  return (
    <>
      <Button size="sm" colorPalette="blue" onClick={() => setOpen(true)}>
        Open dialog
      </Button>
      <EntityDialog
        open={open}
        onClose={() => setOpen(false)}
        title="Edit product"
        footer={
          <HStack justify="flex-end" gap={2}>
            <Button variant="ghost" onClick={() => setOpen(false)}>
              Cancel
            </Button>
            <Button colorPalette="blue" onClick={() => setOpen(false)}>
              Save
            </Button>
          </HStack>
        }
      >
        <Text fontSize="sm" color="fg.muted">
          Same prop API as EntityDrawer — swap one for the other.
        </Text>
      </EntityDialog>
    </>
  );
}

export function ConfirmDialogDemo() {
  const [pending, setPending] = useState<string | null>(null);
  return (
    <>
      <Button size="sm" colorPalette="red" variant="outline" onClick={() => setPending("SUP-0001")}>
        Archive supplier
      </Button>
      <ConfirmDialog
        open={pending != null}
        title="Archive supplier?"
        body={`${pending ?? ""} will stop appearing in pickers. Existing records keep it.`}
        confirmLabel="Archive"
        onConfirm={() => {
          toast.success("Archived", pending ?? "");
          setPending(null);
        }}
        onCancel={() => setPending(null)}
      />
    </>
  );
}

// --- Data display ---------------------------------------------------------

// 24 fixture lots — enough to overflow the demo's cap so the header actually
// sticks and the vertical scroll is visible in the preview.
const SCROLL_ROWS = Array.from({ length: 24 }, (_, i) => ({
  product: `Paracetamol 500mg`,
  batch: `B-2026-${String(i + 1).padStart(4, "0")}`,
  expiry: isoInDays(30 * (i + 1)),
  qty: 120 - i * 3,
}));

export function TableScrollDemo() {
  return (
    <Stack gap={2}>
      <TableScroll maxH="220px">
        <Table.Root size="sm" stickyHeader>
          <Table.Header>
            <Table.Row>
              <Table.ColumnHeader>Product</Table.ColumnHeader>
              <Table.ColumnHeader>Batch no</Table.ColumnHeader>
              <Table.ColumnHeader>Expiry</Table.ColumnHeader>
              <Table.ColumnHeader textAlign="end">Qty</Table.ColumnHeader>
            </Table.Row>
          </Table.Header>
          <Table.Body>
            {SCROLL_ROWS.map((r) => (
              <Table.Row key={r.batch}>
                <Table.Cell>{r.product}</Table.Cell>
                <Table.Cell fontFamily="mono">{r.batch}</Table.Cell>
                <Table.Cell>{r.expiry}</Table.Cell>
                <Table.Cell textAlign="end">{r.qty}</Table.Cell>
              </Table.Row>
            ))}
          </Table.Body>
        </Table.Root>
      </TableScroll>
      <Text fontSize="xs" color="fg.muted">
        Scroll the table — the header stays. The page behind it never moves.
      </Text>
    </Stack>
  );
}

export function PaginationDemo() {
  const { page, setPage, pageSize, setPageSize } = usePageState("demo");
  return (
    <Pagination
      page={page}
      pageSize={pageSize}
      total={137}
      onPageChange={setPage}
      onPageSizeChange={setPageSize}
    />
  );
}

const METRIC_IDS = ["2026-07-28", "2026-07-29", "2026-07-30"];
const METRIC_ORDER = new MetricOrder({
  data: {
    "2026-07-28": { terjual: 1_250_000n, hpp: 800_000n, profit: 450_000n, avgSold: 12n },
    "2026-07-29": { terjual: 2_100_000n, hpp: 1_340_000n, profit: 760_000n, avgSold: 19n },
    "2026-07-30": { terjual: 880_000n, hpp: 610_000n, profit: 270_000n, avgSold: 8n },
  },
});
const METRIC_STOCK = new MetricStock({
  data: {
    "2026-07-28": { ready: 640n, ongoing: 120n, expiring: 0n },
    "2026-07-29": { ready: 590n, ongoing: 120n, expiring: 24n },
    "2026-07-30": { ready: 540n, ongoing: 60n, expiring: 24n },
  },
});

export function MetricTableDemo() {
  const { t } = useTranslation();
  const [sort, setSort] = useState<Sort | undefined>(undefined);
  return (
    <MetricTable
      ids={METRIC_IDS}
      order={METRIC_ORDER}
      stock={METRIC_STOCK}
      metricTypes={[MetricType.ORDER, MetricType.STOCK]}
      labelById={new Map(METRIC_IDS.map((d) => [d, d]))}
      sort={sort}
      onSortChange={setSort}
      dimensionHeader={t("analytics.menu.daily")}
    />
  );
}

const TREND_DATA = [
  { day: "2026-07-24", revenue: 1_250_000, profit: 450_000 },
  { day: "2026-07-25", revenue: 2_100_000, profit: 760_000 },
  { day: "2026-07-26", revenue: 880_000, profit: 270_000 },
  { day: "2026-07-27", revenue: 1_640_000, profit: 520_000 },
  { day: "2026-07-28", revenue: 1_980_000, profit: 690_000 },
  { day: "2026-07-29", revenue: 2_540_000, profit: 910_000 },
  { day: "2026-07-30", revenue: 1_420_000, profit: 380_000 },
];

export function ChartCardDemo() {
  return (
    <SimpleGrid columns={{ base: 1, md: 2 }} gap={4}>
      <ChartCard title="Revenue" description="Last 7 days" height="180px">
        <TrendChart data={TREND_DATA} xKey="day" money series={[{ dataKey: "revenue", label: "Revenue" }]} />
      </ChartCard>
      <ChartCard title="Revenue" description="No data in range" height="180px" isEmpty>
        <span />
      </ChartCard>
    </SimpleGrid>
  );
}

export function TrendChartDemo() {
  const [multi, setMulti] = useState(false);
  return (
    <Stack gap={3}>
      <Button size="sm" variant="outline" alignSelf="flex-start" onClick={() => setMulti((v) => !v)}>
        {multi ? "single series (filled area)" : "two series (lines + legend)"}
      </Button>
      <ChartCard title="Revenue vs profit" height="220px">
        <TrendChart
          data={TREND_DATA}
          xKey="day"
          money
          series={
            multi
              ? [
                  { dataKey: "revenue", label: "Revenue" },
                  { dataKey: "profit", label: "Profit" },
                ]
              : [{ dataKey: "revenue", label: "Revenue" }]
          }
        />
      </ChartCard>
    </Stack>
  );
}

export function ExpiryBadgeDemo() {
  return (
    <HStack gap={3} flexWrap="wrap">
      <ExpiryBadge expiry={isoInDays(-3)} />
      <ExpiryBadge expiry={isoInDays(12)} />
      <ExpiryBadge expiry={isoInDays(60)} />
      <ExpiryBadge expiry={isoInDays(200)} />
      <ExpiryBadge expiry={isoInDays(500)} />
    </HStack>
  );
}

export function UserAvatarDemo() {
  // version={0} means "no picture", which disables the fetch — so this demo
  // renders the initials fallback at every size and makes no server call.
  return (
    <HStack gap={4} align="center" flexWrap="wrap">
      <UserAvatar userId="demo-1" name="Siti Rahmawati" version={0} size="xs" />
      <UserAvatar userId="demo-2" name="Budi" version={0} size="sm" />
      <UserAvatar userId="demo-3" name="Andi Pratama" version={0} size="md" />
      <UserAvatar userId="demo-4" name="Dewi Lestari" version={0} size="lg" />
      <UserAvatar userId="demo-5" name="Eko" version={0} size="xl" />
    </HStack>
  );
}

export function ProductImageDemo() {
  // version={0} means "no picture", which disables the fetch — so this demo
  // renders the placeholder at every size and makes no server call.
  return (
    <HStack gap={4} align="center" flexWrap="wrap">
      <ProductImage productId="demo-1" name="Paracetamol 500mg" version={0} size={28} />
      <ProductImage productId="demo-2" name="Amoxicillin" version={0} size={36} />
      <ProductImage productId="demo-3" name="Vitamin C" version={0} size={56} />
      <ProductImage productId="demo-4" name="Antasida" version={0} size={96} />
    </HStack>
  );
}

// --- Toolbar & filters ----------------------------------------------------

export function DateRangeFilterDemo() {
  const { t } = useTranslation();
  const [range, setRange] = useState<DateRange>(() => resolveRange("30d"));
  const { from, to } = rangeBounds(range);
  // Second instance: a list with several filterable dates. Sample field labels
  // — a real page passes translated ones.
  const [fieldRange, setFieldRange] = useState<DateRange>(() => resolveRange("30d"));
  const [field, setField] = useState("");
  return (
    <Stack gap={5} align="flex-start">
      <Stack gap={2} align="flex-start">
        <Text fontSize="xs" color="fg.muted">
          {t("dev.components.demo.rangeOneDate")}
        </Text>
        <DateRangeFilter value={range} onChange={setRange} />
        <Emitted>{`${range.preset} · ${formatAbsolute(from)} → ${formatAbsolute(to)}`}</Emitted>
        <Emitted>{`fromUnix ${range.fromUnix} · toUnix ${range.toUnix}`}</Emitted>
      </Stack>

      <Stack gap={2} align="flex-start">
        <Text fontSize="xs" color="fg.muted">
          {t("dev.components.demo.rangeManyDates")}
        </Text>
        <DateRangeFilter
          value={fieldRange}
          onChange={setFieldRange}
          fields={[
            { value: "created", label: "Created" },
            { value: "received", label: "Received" },
            { value: "voided", label: "Voided" },
          ]}
          field={field}
          onFieldChange={setField}
        />
        <Emitted>{`dateField "${field}"`}</Emitted>
        {/* Any date sends no bounds at all — that's the caller's half of the contract. */}
        <Emitted>
          {field
            ? `fromUnix ${fieldRange.fromUnix} · toUnix ${fieldRange.toUnix}`
            : "fromUnix 0 · toUnix 0"}
        </Emitted>
      </Stack>
    </Stack>
  );
}

const COLUMN_GROUPS = [
  {
    id: "order",
    label: "Order",
    fields: [
      { id: "order.terjual", label: "Terjual" },
      { id: "order.hpp", label: "HPP" },
      { id: "order.profit", label: "Profit" },
    ],
  },
  {
    id: "stock",
    label: "Stock",
    fields: [
      { id: "stock.ready", label: "Ready" },
      { id: "stock.ongoing", label: "Ongoing" },
    ],
  },
];

export function ColumnsPopoverDemo() {
  const [visible, setVisible] = useState<Set<string>>(
    () => new Set(["order.terjual", "order.profit", "stock.ready"]),
  );
  return (
    <Stack gap={2} align="flex-start">
      <ColumnsPopover value={visible} onChange={setVisible} groups={COLUMN_GROUPS} />
      <Emitted>{[...visible].sort().join(", ")}</Emitted>
    </Stack>
  );
}

const UNIT_GROUPS: StockUnitGroup[] = [
  { baseName: "tablet", derivatives: ["strip", "box"] },
  { baseName: "ml", derivatives: ["botol"] },
];

export function StockUnitPopoverDemo() {
  const [byBase, setByBase] = useState<StockUnitsByBase>({ tablet: "strip" });
  return (
    <Stack gap={2} align="flex-start">
      <StockUnitPopover
        groups={UNIT_GROUPS}
        byBase={byBase}
        onChangeBase={(baseName, deriv) => setByBase((prev) => ({ ...prev, [baseName]: deriv }))}
      />
      <Emitted>{JSON.stringify(byBase)}</Emitted>
    </Stack>
  );
}

export function ExportButtonDemo() {
  return (
    <ExportButton
      onExport={async () => {
        await new Promise((resolve) => setTimeout(resolve, 900));
        toast.success("Exported", "3 rows");
      }}
    />
  );
}

// --- Feedback ------------------------------------------------------------

export function ToastDemo() {
  return (
    <HStack gap={2} flexWrap="wrap">
      <Button size="sm" variant="outline" onClick={() => toast.success("Saved", "Supplier created")}>
        success
      </Button>
      <Button size="sm" variant="outline" onClick={() => toast.info("Heads up", "Draft restored")}>
        info
      </Button>
      <Button size="sm" variant="outline" onClick={() => toast.error("Failed", "Insufficient stock")}>
        error
      </Button>
      <Button
        size="sm"
        variant="outline"
        onClick={() => toast.fromError(new Error("product.sku_taken"))}
      >
        fromError
      </Button>
    </HStack>
  );
}

export function BarcodeDemo() {
  return (
    <HStack gap={6} flexWrap="wrap" align="flex-start">
      <Barcode value="SKU-00123" />
      <Barcode value="OBAT-88219" height={40} width={1.5} fontSize={11} />
      {/* Non-ASCII: renders the warning rather than a symbol that looks
          scannable but isn't. */}
      <Barcode value="obat-é" />
    </HStack>
  );
}
