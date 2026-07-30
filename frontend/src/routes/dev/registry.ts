import {
  Layers,
  LayoutTemplate,
  ListFilter,
  MessageSquare,
  SlidersHorizontal,
  Table2,
  TextCursorInput,
} from "lucide-react";
import type { ComponentType } from "react";

import * as demo from "./demos";

// Registry for the dev-only component gallery (/components).
//
// This is the curated list of SHARED components — the app's style vocabulary.
// A component belongs here when it is meant to be reused across pages; a
// page-local piece of JSX does not. When you add a shared component to
// src/components/, add an entry here in the same change so the gallery stays
// the source of truth for "what already exists" (and nobody hand-rolls a
// second MoneyInput).
//
// Grouping is by CONTEXT (what you reach for it during), not by file name —
// that's what makes the left rail scannable.

export type PropDoc = {
  name: string;
  type: string;
  required?: boolean;
  /** Developer-facing note. Not user copy, so it stays in English (like a code comment). */
  desc: string;
};

export type ComponentEntry = {
  /** URL fragment + scroll anchor. Kebab-case. */
  id: string;
  /** The component's exported name (a code identifier — never translated). */
  name: string;
  /** Repo-relative source path, shown as the import hint. */
  file: string;
  /** One-line "reach for this when…". */
  summary: string;
  props: PropDoc[];
  /** Copy-pasteable call site. */
  usage: string;
  /** Optional gotcha / hard-rule reminder. */
  notes?: string;
  Demo: ComponentType;
};

export type ComponentGroup = {
  id: string;
  /** i18n key under `dev.components.groups`. */
  labelKey: string;
  /** A lucide icon (same `typeof <Icon>` shape the Sidebar uses). */
  icon: typeof Layers;
  entries: ComponentEntry[];
};

export const GROUPS: ComponentGroup[] = [
  {
    id: "layout",
    labelKey: "layout",
    icon: LayoutTemplate,
    entries: [
      {
        id: "page-header",
        name: "PageHeader",
        file: "src/components/PageHeader.tsx",
        summary: "Every page starts with this: breadcrumbs, title, description, right-aligned actions.",
        props: [
          { name: "title", type: "string", required: true, desc: "Page title (already localized)." },
          { name: "breadcrumbs", type: "{ label: string; to?: string }[]", desc: "Crumb trail; `to` makes it a client-side link." },
          { name: "description", type: "string", desc: "Muted sub-line under the title." },
          { name: "actions", type: "ReactNode", desc: "Right-aligned action buttons." },
        ],
        usage: `<PageHeader
  breadcrumbs={[{ label: t("nav.inventory"), to: "/inventory/batches" }]}
  title={t("inventory.tabs.batches")}
  actions={<Button colorPalette="blue">{t("common.add")}</Button>}
/>`,
        Demo: demo.PageHeaderDemo,
      },
      {
        id: "route-tabs",
        name: "RouteTabs",
        file: "src/components/RouteTabs.tsx",
        summary: "URL-driven tab strip — one route per tab, rendered above an <Outlet/>.",
        props: [
          { name: "items", type: "{ value: string; label: string; to: string }[]", required: true, desc: "Tabs; active one is the longest `to` prefix of the current path." },
        ],
        usage: `<RouteTabs items={[
  { value: "daily", to: "/analytics/daily", label: t("analytics.menu.daily") },
  { value: "product", to: "/analytics/product", label: t("analytics.menu.product") },
]} />
<Outlet />`,
        notes: "Tabs are buttons, not anchors (an <a href> beat NavLink's SPA handler and full-reloaded the page). For tabs that share one route, use Chakra Tabs.Root directly.",
        Demo: demo.RouteTabsDemo,
      },
      {
        id: "back-button",
        name: "BackButton",
        file: "src/components/BackButton.tsx",
        summary: "Detail-page back nav. HARD RULE: every :id page renders one.",
        props: [
          { name: "to", type: "string", desc: "Parent list path. Omit to fall back to history (navigate(-1))." },
          { name: "label", type: "string", desc: "Overrides the default t(\"common.back\")." },
        ],
        usage: `<BackButton to="/products" />`,
        Demo: demo.BackButtonDemo,
      },
      {
        id: "dashboard-tile",
        name: "DashboardTile",
        file: "src/components/DashboardTile.tsx",
        summary: "KPI tile for the role-keyed Dashboard sections; clickable when `to` is set.",
        props: [
          { name: "label", type: "string", required: true, desc: "Metric name." },
          { name: "value", type: "string", required: true, desc: "Pre-formatted value (money via formatMoney)." },
          { name: "hint", type: "string", desc: "Small line under the value." },
          { name: "to", type: "string", desc: "Makes the whole tile a router link." },
          { name: "tone", type: '"default" | "warning" | "danger"', desc: "Colors border + value for alert states." },
        ],
        usage: `<DashboardTile
  label={t("dashboard.lowStock")}
  value={String(count)}
  tone={count > 0 ? "warning" : "default"}
  to="/products"
/>`,
        Demo: demo.DashboardTileDemo,
      },
    ],
  },
  {
    id: "forms",
    labelKey: "forms",
    icon: TextCursorInput,
    entries: [
      {
        id: "form-field",
        name: "FormField",
        file: "src/components/FormField.tsx",
        summary: "The only way we render a form input: RHF Controller + Chakra Field + translated error text.",
        props: [
          { name: "control", type: "Control<TForm>", required: true, desc: "From useForm()." },
          { name: "name", type: "FieldPath<TForm>", required: true, desc: "Schema field name." },
          { name: "label", type: "string", required: true, desc: "Localized label." },
          { name: "required", type: "boolean", desc: "Renders the required indicator." },
          { name: "type", type: 'HTML input type', desc: '"date" swaps in the shared DatePicker (not native chrome).' },
          { name: "money", type: "boolean", desc: "Renders MoneyInput (emits a raw digit string)." },
          { name: "number", type: "boolean", desc: "Renders NumberInput (digits only)." },
          { name: "passwordToggle", type: "boolean", desc: "Eye show/hide, with type=\"password\"." },
          { name: "disabled", type: "boolean", desc: "Read-only immutable field (still in the schema)." },
          { name: "helperText", type: "string", desc: "Shown until an error replaces it." },
        ],
        usage: `const Schema = z.object({ name: z.string().min(1), price: z.coerce.bigint() });
const form = useForm<z.infer<typeof Schema>>({ resolver: zodResolver(Schema) });

<FormField control={form.control} name="name" label={t("products.name")} required />
<FormField control={form.control} name="price" label={t("products.price")} money />`,
        notes: "Schemas declare PLAIN rules with no inline messages — the global Zod error map translates them. Never hand-roll useState + red <Text> validation.",
        Demo: demo.FormFieldDemo,
      },
      {
        id: "money-input",
        name: "MoneyInput",
        file: "src/components/MoneyInput.tsx",
        summary: "Integer-money input: locale thousands grouping in, raw digit string out.",
        props: [
          { name: "value", type: "number | bigint | string | null", required: true, desc: "Minor units. 0 renders empty (placeholder)." },
          { name: "onChange", type: "(raw: string) => void", required: true, desc: 'Unformatted digits; "" for empty/zero.' },
          { name: "size / width / disabled / autoFocus / onBlur", type: "—", desc: "Passed through to Chakra Input." },
        ],
        usage: `<MoneyInput value={price} onChange={(raw) => setPrice(Number(raw || 0))} />`,
        notes: "In a form, pass `money` to FormField instead of mounting this directly. Never use <Input type=\"number\"> for money.",
        Demo: demo.MoneyInputDemo,
      },
      {
        id: "number-input",
        name: "NumberInput",
        file: "src/components/NumberInput.tsx",
        summary: "Digits-only quantity input; same emit contract as MoneyInput, no grouping.",
        props: [
          { name: "value", type: "number | bigint | string | null", required: true, desc: "Leading zeros stripped; 0 renders empty." },
          { name: "onChange", type: "(raw: string) => void", required: true, desc: 'Digit string, "" for empty/zero.' },
          { name: "max", type: "number", desc: "Clamps the emitted value (typed overflow snaps to max)." },
        ],
        usage: `<NumberInput value={qty} onChange={(raw) => setQty(Number(raw || 0))} max={stock} />`,
        Demo: demo.NumberInputDemo,
      },
      {
        id: "date-picker",
        name: "DatePicker",
        file: "src/components/DatePicker.tsx",
        summary: "Calendar popover (day/month/year views) replacing native <input type=\"date\">.",
        props: [
          { name: "value", type: "string | null", required: true, desc: "YYYY-MM-DD, or \"\" for empty." },
          { name: "onChange", type: "(value: string) => void", required: true, desc: 'YYYY-MM-DD out, "" when cleared.' },
          { name: "min / max", type: "string", desc: "YYYY-MM-DD bounds." },
          { name: "clearable", type: "boolean", desc: "Inline × button. Default true." },
        ],
        usage: `<DatePickerField value={expiry} onChange={setExpiry} min={today} />`,
        notes: "Popover is portalled, so it works inside EntityDrawer / Dialog. Derives the ISO date from the DateValue, not the locale-formatted string.",
        Demo: demo.DatePickerDemo,
      },
      {
        id: "discount-field",
        name: "DiscountField",
        file: "src/components/DiscountField.tsx",
        summary: "Rp/% toggle + value input — the shared discount affordance (POS lines, cart, purchasing).",
        props: [
          { name: "type", type: '"FIXED" | "PERCENT"', required: true, desc: "Which input renders." },
          { name: "value", type: "number", required: true, desc: "HUMAN units: minor units for FIXED, plain percent for PERCENT." },
          { name: "onChange", type: "(type, value) => void", required: true, desc: "Switching type resets the value to 0." },
        ],
        usage: `<DiscountField type={type} value={value} onChange={(t, v) => setDiscount(t, v)} />`,
        notes: "Callers convert PERCENT to basis points (×100) before sending it to the backend.",
        Demo: demo.DiscountFieldDemo,
      },
    ],
  },
  {
    id: "selects",
    labelKey: "selects",
    icon: ListFilter,
    entries: [
      {
        id: "enum-select",
        name: "EnumSelect",
        file: "src/components/EnumSelect.tsx",
        summary: "Chakra Select wrapper for short fixed enums (status, role, payment source, page size).",
        props: [
          { name: "value / onChange", type: "string | null / (v: string) => void", required: true, desc: "Single-select by design." },
          { name: "items", type: "readonly T[]", required: true, desc: "≤ ~20 static options." },
          { name: "itemToString / itemToValue", type: "(item: T) => string", required: true, desc: "Label + value projections." },
          { name: "placeholder / disabled / size / width", type: "—", desc: "Same API as SearchableSelect." },
        ],
        usage: `<EnumSelect
  value={status}
  onChange={setStatus}
  items={STATUS_OPTIONS}
  itemToString={(o) => o.label}
  itemToValue={(o) => o.value}
/>`,
        notes: "NativeSelect is banned — OS chrome clashes with the Chakra UI.",
        Demo: demo.EnumSelectDemo,
      },
      {
        id: "searchable-select",
        name: "SearchableSelect",
        file: "src/components/SearchableSelect.tsx",
        summary: "Combobox wrapper with type-to-filter. Two modes: async loadOptions (dynamic data) or sync items.",
        props: [
          { name: "value / onChange", type: "string | null / (v: string) => void", required: true, desc: "Selected id." },
          { name: "loadOptions", type: "(query: string) => Promise<readonly T[]>", desc: "Async mode: debounced 250ms + stale-result guard. REQUIRED for dynamic domains." },
          { name: "items", type: "readonly T[]", desc: "Sync mode: ≤20 static options only." },
          { name: "itemToString / itemToValue", type: "(item: T) => string", required: true, desc: "Label + value projections." },
          { name: "selectedLabel", type: "string", desc: "Trigger label before the first search resolves (edit drawers)." },
          { name: "onSelectItem", type: "(item: T | undefined) => void", desc: "Hands back the full picked object alongside onChange." },
        ],
        usage: `<SearchableSelect
  value={productId}
  onChange={setProductId}
  loadOptions={searchProducts}
  itemToString={(p) => p.name}
  itemToValue={(p) => p.id}
  selectedLabel={record?.productName}
/>`,
        notes: "HARD RULE: options from a queryable domain (products, customers, suppliers, batches…) must come from a Search<Domain> RPC via loadOptions — never a full-list preload.",
        Demo: demo.SearchableSelectDemo,
      },
      {
        id: "warehouse-select",
        name: "WarehouseSelect",
        file: "src/components/WarehouseSelect.tsx",
        summary: "Warehouse picker as a button → searchable modal. Used by TopBar, POS header, Transfers.",
        props: [
          { name: "value / onChange", type: "string / (id: string) => void", required: true, desc: "Warehouse id." },
          { name: "warehouses", type: "readonly Warehouse[]", desc: "Static mode: filtered in-memory." },
          { name: "loadOptions", type: "(query: string) => Promise<Warehouse[]>", desc: "Async mode (TopBar): debounced server search." },
          { name: "excludeId", type: "string", desc: "Hides one option (Transfer \"To\" hides the chosen \"From\")." },
          { name: "selectedLabel", type: "string", desc: "Chip label when the picked row isn't in the loaded list." },
        ],
        usage: `<WarehouseSelect warehouses={warehousesQ.rows} value={fromId} onChange={setFromId} />`,
        Demo: demo.WarehouseSelectDemo,
      },
    ],
  },
  {
    id: "overlays",
    labelKey: "overlays",
    icon: Layers,
    entries: [
      {
        id: "entity-drawer",
        name: "EntityDrawer",
        file: "src/components/EntityDrawer.tsx",
        summary: "Slide-over from the right — the default create/edit surface.",
        props: [
          { name: "open / onClose", type: "boolean / () => void", required: true, desc: "Controlled; closing on backdrop/Esc routes through onClose." },
          { name: "title", type: "string", required: true, desc: "Header heading." },
          { name: "children", type: "ReactNode", required: true, desc: "Body (auto-stacked, gap 4)." },
          { name: "footer", type: "ReactNode", desc: "Sticky footer — put Cancel/Save here." },
          { name: "size", type: '"sm" | "md" | "lg" | "xl"', desc: "Default md." },
        ],
        usage: `<EntityDrawer open={open} onClose={close} title={t("customers.add")} footer={<Actions />}>
  <FormField … />
</EntityDrawer>`,
        notes: "Keep it MOUNTED and drive `open` — never `{open && <EntityDrawer/>}` or `if (!open) return null`. Unmounting while open leaves Ark's body lock in place and freezes the page.",
        Demo: demo.EntityDrawerDemo,
      },
      {
        id: "entity-dialog",
        name: "EntityDialog",
        file: "src/components/EntityDialog.tsx",
        summary: "Centered-modal counterpart of EntityDrawer — identical prop API (used by the product form).",
        props: [
          { name: "open / onClose / title / children / footer / size", type: "—", required: true, desc: "Same as EntityDrawer, so callers can swap one for the other." },
        ],
        usage: `<EntityDialog open={open} onClose={close} title={t("products.edit")} footer={<Actions />}>
  <FormField … />
</EntityDialog>`,
        Demo: demo.EntityDialogDemo,
      },
      {
        id: "confirm-dialog",
        name: "ConfirmDialog",
        file: "src/components/ConfirmDialog.tsx",
        summary: "The only confirmation affordance — archive, void, delete, set-default.",
        props: [
          { name: "open", type: "boolean", required: true, desc: "Drive from a `pending` state (open={pending != null})." },
          { name: "title / confirmLabel", type: "string", required: true, desc: "Localized copy." },
          { name: "body", type: "ReactNode", desc: "String renders as muted text." },
          { name: "confirmColorPalette", type: "string", desc: 'Default "red" (destructive).' },
          { name: "loading", type: "boolean", desc: "Spinner on the confirm button while the mutation runs." },
          { name: "onConfirm / onCancel", type: "() => void", required: true, desc: "Cancel also fires on backdrop / Esc / ×." },
        ],
        usage: `<ConfirmDialog
  open={pending != null}
  title={t("suppliers.archiveTitle")}
  confirmLabel={t("common.archive")}
  loading={archive.isPending}
  onConfirm={() => archive.mutate(pending!)}
  onCancel={() => setPending(null)}
/>`,
        notes: "HARD RULE: never window.confirm / alert / prompt.",
        Demo: demo.ConfirmDialogDemo,
      },
    ],
  },
  {
    id: "data",
    labelKey: "data",
    icon: Table2,
    entries: [
      {
        id: "pagination",
        name: "Pagination",
        file: "src/components/Pagination.tsx",
        summary: "\"Showing X–Y of N\" + Prev/Next + page size. Render under every server-paginated table.",
        props: [
          { name: "page", type: "number", required: true, desc: "0-based." },
          { name: "pageSize / total", type: "number", required: true, desc: "total = unfiltered-by-page count from the List RPC." },
          { name: "onPageChange", type: "(page: number) => void", required: true, desc: "From usePageState." },
          { name: "onPageSizeChange", type: "(size: number) => void", desc: "Omit to hide the size picker." },
        ],
        usage: `const { page, setPage, pageSize, setPageSize } = usePageState(filterKey);
const q = useProductsQuery({ page, pageSize });
…
<Pagination page={page} pageSize={pageSize} total={q.total}
  onPageChange={setPage} onPageSizeChange={setPageSize} />`,
        notes: "Pair with usePageState(resetKey) so changing a filter snaps back to page 0.",
        Demo: demo.PaginationDemo,
      },
      {
        id: "metric-table",
        name: "MetricTable",
        file: "src/components/MetricTable.tsx",
        summary: "Analytics grid: grouped headers, click-to-sort columns, ids → labels via a resolve map.",
        props: [
          { name: "ids", type: "string[]", required: true, desc: "Server order (already sorted)." },
          { name: "order / stock", type: "MetricOrder / MetricStock", desc: "Metric blocks keyed by id." },
          { name: "metricTypes", type: "MetricType[]", required: true, desc: "Which column groups the backend was asked for." },
          { name: "visibleFields", type: "Set<string>", desc: 'Per-column visibility ("order.terjual", …) from ColumnsPopover.' },
          { name: "labelById", type: "Map<string, string>", required: true, desc: "Display names; missing → \"—\"." },
          { name: "sort / onSortChange", type: "Sort | undefined", desc: "Header click cycles unsorted → DESC → ASC." },
          { name: "dimensionHeader", type: "string", required: true, desc: "First column header." },
          { name: "isLoading", type: "boolean", desc: "Renders an in-table spinner row." },
        ],
        usage: `<MetricTable
  ids={q.data?.days ?? []}
  order={q.data?.order}
  metricTypes={metricTypes}
  visibleFields={visible}
  labelById={labels}
  sort={sort}
  onSortChange={setSort}
  dimensionHeader={t("analytics.menu.daily")}
/>`,
        Demo: demo.MetricTableDemo,
      },
      {
        id: "expiry-badge",
        name: "ExpiryBadge",
        file: "src/components/ExpiryBadge.tsx",
        summary: "Days-to-expiry badge: ≤30d red, ≤90d orange, else green; relative localized label.",
        props: [
          { name: "expiry", type: "string", required: true, desc: "Date parseable by new Date() (YYYY-MM-DD)." },
        ],
        usage: `<ExpiryBadge expiry={batch.expiryDate} />`,
        Demo: demo.ExpiryBadgeDemo,
      },
    ],
  },
  {
    id: "toolbar",
    labelKey: "toolbar",
    icon: SlidersHorizontal,
    entries: [
      {
        id: "date-range-filter",
        name: "DateRangeFilter",
        file: "src/components/DateRangeFilter.tsx",
        summary: "Preset range picker (Today / 7d / 30d / 90d / YTD / custom) resolving to unix bounds.",
        props: [
          { name: "value", type: "DateRange", required: true, desc: "{ preset, fromUnix, toUnix, customFrom?, customTo? }." },
          { name: "onChange", type: "(next: DateRange) => void", required: true, desc: "Already resolved — send fromUnix/toUnix straight to the RPC." },
        ],
        usage: `const [range, setRange] = useState<DateRange>(() => resolveRange("30d"));
<DateRangeFilter value={range} onChange={setRange} />`,
        notes: "resolveRange(preset, customFrom?, customTo?) is exported for deriving the initial value.",
        Demo: demo.DateRangeFilterDemo,
      },
      {
        id: "columns-popover",
        name: "ColumnsPopover",
        file: "src/components/ColumnsPopover.tsx",
        summary: "One button → sectioned checkboxes for per-field column visibility, with a counter + Reset.",
        props: [
          { name: "value / onChange", type: "Set<string>", required: true, desc: 'Field ids, "<group>.<field>". Caller owns the state.' },
          { name: "groups", type: "GroupSpec[]", required: true, desc: "{ id, label, fields[], disabled?, disabledReason? } — a whole group can be disabled." },
          { name: "defaults", type: "Set<string>", desc: "What Reset restores. Defaults to every enabled field." },
        ],
        usage: `<ColumnsPopover value={visible} onChange={setVisible} groups={FIELD_GROUPS} />`,
        Demo: demo.ColumnsPopoverDemo,
      },
      {
        id: "stock-unit-popover",
        name: "StockUnitPopover",
        file: "src/components/StockUnitPopover.tsx",
        summary: "Per-base-unit radio groups picking how stock quantities render (base count vs strip/box).",
        props: [
          { name: "byBase", type: "StockUnitsByBase", required: true, desc: 'baseName → derivative ("" = raw base count).' },
          { name: "onChangeBase", type: "(baseName, deriv) => void", required: true, desc: "Persisted in usePreferencesStore by the caller." },
          { name: "groups", type: "StockUnitGroup[]", required: true, desc: "From unitGroupsFromCatalog(catalog, pageRows)." },
        ],
        usage: `<StockUnitPopover groups={groups} byBase={byBase} onChangeBase={setStockUnit} />`,
        Demo: demo.StockUnitPopoverDemo,
      },
      {
        id: "export-button",
        name: "ExportButton",
        file: "src/components/ExportButton.tsx",
        summary: "\"Export CSV\" with a loading state; failures go to the global toast.",
        props: [
          { name: "onExport", type: "() => Promise<void>", required: true, desc: "Fetch all matching rows, serialize, trigger the download." },
          { name: "disabled / size", type: "—", desc: "Passed through." },
        ],
        usage: `<ExportButton onExport={async () => downloadCsv("orders.csv", await fetchAllRows())} />`,
        Demo: demo.ExportButtonDemo,
      },
    ],
  },
  {
    id: "feedback",
    labelKey: "feedback",
    icon: MessageSquare,
    entries: [
      {
        id: "toast",
        name: "toast",
        file: "src/lib/toaster.tsx",
        summary: "Global toaster. Unhandled query/mutation errors already route here automatically.",
        props: [
          { name: "toast.success(title, description?)", type: "void", desc: "Confirmation after a mutation." },
          { name: "toast.info(title, description?)", type: "void", desc: "Neutral notice." },
          { name: "toast.error(title, description?)", type: "void", desc: "Handled failure." },
          { name: "toast.fromError(err, fallbackTitle?)", type: "void", desc: "Translates backend error tokens; genericizes unknown raw strings." },
        ],
        usage: `import { toast } from "../lib/toaster";

toast.success(t("customers.created"));
// opt a mutation out of the automatic toast:
useMutation({ …, meta: { silentError: true } });`,
        notes: "Field-attributable server errors belong on the field via useServerFormErrors, not in a toast.",
        Demo: demo.ToastDemo,
      },
    ],
  },
];

export const DEFAULT_GROUP = GROUPS[0].id;

export function findGroup(id: string | undefined): ComponentGroup | undefined {
  return GROUPS.find((g) => g.id === id);
}

export const TOTAL_ENTRIES = GROUPS.reduce((n, g) => n + g.entries.length, 0);
