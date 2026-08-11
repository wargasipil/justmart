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
        summary: "Every page starts with this: title, optional badge + description, right-aligned actions.",
        props: [
          { name: "title", type: "string", required: true, desc: "Page title (already localized)." },
          { name: "titleBadge", type: "ReactNode", desc: "Inline slot right after the title — put a status Badge (active/archived) here, not in the page body." },
          { name: "description", type: "string", desc: "Muted sub-line under the title." },
          { name: "actions", type: "ReactNode", desc: "Right-aligned action buttons." },
        ],
        usage: `<PageHeader
  title={t("inventory.tabs.batches")}
  actions={<Button colorPalette="blue">{t("common.add")}</Button>}
/>`,
        notes: "No `breadcrumbs` prop — the trail lives once in the TopBar (<Breadcrumbs/>) and is derived from the URL. A detail page contributes only its entity name, via useCrumbLabel().",
        Demo: demo.PageHeaderDemo,
      },
      {
        id: "breadcrumbs",
        name: "Breadcrumbs",
        file: "src/components/Breadcrumbs.tsx",
        summary: "The app-wide \"you are here\" trail. Mounted once in the TopBar — never rendered by a page.",
        props: [
          { name: "—", type: "no props", desc: "The trail comes from the URL via the registry in src/lib/breadcrumbs.ts." },
        ],
        usage: `// A new route gets a crumb by adding one entry to ROUTES in lib/breadcrumbs.ts:
{ path: "/inventory/batches", labelKey: "inventory.tabs.batches", parent: "@inventory" }

// A detail page supplies its own leaf label:
useCrumbLabel(productQ.data?.name)`,
        notes: "Ancestors mirror the sidebar, so a \"@\"-prefixed entry (e.g. @inventory) is a nav group with no page of its own and renders unlinked. A dynamic leaf is omitted until the page registers a name — never a raw UUID. Narrow viewports collapse to the current crumb.",
        Demo: demo.BreadcrumbsDemo,
      },
      {
        id: "route-tabs",
        name: "RouteTabs",
        file: "src/components/RouteTabs.tsx",
        summary: "URL-driven tabs — one route per tab, as a strip above an <Outlet/> or a rail beside it.",
        props: [
          { name: "items", type: "{ value: string; label: string; to: string }[]", required: true, desc: "Tabs; active one is the longest `to` prefix of the current path." },
          { name: "orientation", type: '"horizontal" | "vertical"', desc: 'Default "horizontal" (strip). "vertical" renders a left rail — lay the <Outlet/> out beside it; resolve with useTabsOrientation().' },
        ],
        usage: `<RouteTabs items={[
  { value: "daily", to: "/analytics/daily", label: t("analytics.menu.daily") },
  { value: "product", to: "/analytics/product", label: t("analytics.menu.product") },
]} />
<Outlet />

// Vertical rail (see routes/Settings.tsx) — degrades to a strip below \`md\`:
const orientation = useTabsOrientation();
<Flex direction={orientation === "vertical" ? "row" : "column"} gap={6}>
  <RouteTabs items={tabs} orientation={orientation} />
  <Box flex="1" minW={0}><Outlet /></Box>
</Flex>`,
        notes: "Tabs are buttons, not anchors (an <a href> beat NavLink's SPA handler and full-reloaded the page). For tabs that share one route, use Chakra Tabs.Root directly. Triggers never shrink — a horizontal strip scrolls rather than overlapping its labels. Never hard-code orientation=\"vertical\": `useTabsOrientation()` (lib/tabs.ts) degrades it to a strip on a viewport too narrow for a rail.",
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
        summary: "KPI tile for the role-keyed Dashboard sections; clickable when `to` is set. Chakra Stat under the hood.",
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
      {
        id: "summary-tile",
        name: "SummaryTile",
        file: "src/components/SummaryTile.tsx",
        summary: "Flat tile for a list page's summary bar — the server-side aggregate over ALL filtered rows, above the table. Chakra Stat under the hood.",
        props: [
          { name: "label", type: "string", required: true, desc: "Metric name." },
          { name: "value", type: "string", required: true, desc: "Pre-formatted value (money via formatMoney)." },
        ],
        usage: `<SimpleGrid columns={{ base: 1, sm: 2, lg: 4 }} gap={3}>
  <SummaryTile
    label={t("inventory.products.summary.ready")}
    value={formatCount(summary?.readyStock ?? 0n)}
  />
  <SummaryTile
    label={t("inventory.products.summary.readyValue")}
    value={formatMoney(summary?.readyValuation ?? 0n)}
  />
</SimpleGrid>`,
        notes:
          "One figure per tile — a count and its valuation are two cells, not a value with a subtitle. Feed it a dedicated summary RPC that honors the SAME filters as the list (GetProductsSummary / GetSalesSummary) — never a client-side sum of the page, which would change meaning as the user pages. Use <DashboardTile> instead for a landing-page card (bigger, clickable, tone-colored).",
        Demo: demo.SummaryTileDemo,
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
          { name: "renderItem", type: "(item: T) => ReactElement", desc: "Custom dropdown-row content when one line of text can't tell options apart — e.g. CashierFilterSelect renders the Users-table identity cell (avatar + name over a muted line) with the role on that line. Must return ONE element — mounted via <Combobox.ItemText asChild>. Affects the open list only; itemToString still drives the trigger." },
          { name: "emptyText / loadingText", type: "string", desc: "Override the (already translated) common.noResults / common.loading defaults." },
          { name: "startElement", type: "ReactNode", desc: "Leading glyph inside the control, left of the text (see SupplierSelect). Decorative — pointer-events:none so clicking it still opens the popover, and aria-hidden. Adds ps={8} to the input." },
        ],
        usage: `<SearchableSelect
  value={productId}
  onChange={setProductId}
  loadOptions={searchProducts}
  itemToString={(p) => p.name}
  itemToValue={(p) => p.id}
  selectedLabel={record?.productName}
/>

// Rich rows — avatar + name + role badge:
<SearchableSelect<UserRef>
  value={cashierId || null}
  onChange={setCashierId}
  loadOptions={searchSellingUsers}
  itemToString={(u) => displayName(u)}
  itemToValue={(u) => u.id}
  renderItem={(u) => <CashierOption user={u} />}
/>`,
        notes: "HARD RULE: options from a queryable domain (products, customers, suppliers, batches…) must come from a Search<Domain> RPC via loadOptions — never a full-list preload. renderItem returns a single element because it is mounted with asChild — returning a fragment or two siblings throws. A stub row (pre-set value not yet loaded) has no original item and falls back to the plain label.",
        Demo: demo.SearchableSelectDemo,
      },
      {
        id: "supplier-select",
        name: "SupplierSelect",
        file: "src/components/SupplierSelect.tsx",
        summary: "Supplier picker: SearchableSelect pre-wired to searchSuppliers, with a Building2 glyph and two-line rows.",
        props: [
          { name: "value", type: "string", required: true, desc: "Supplier id (\"\" = none)." },
          { name: "onChange", type: "(id: string) => void", desc: "Omit for a read-only display (pair with disabled)." },
          { name: "onSelectItem", type: "(s: Supplier | undefined) => void", desc: "Hands back the full picked supplier." },
          { name: "selectedLabel", type: "string", desc: "Escape hatch — skips the internal resolve when the caller already has the supplier loaded." },
          { name: "placeholder / disabled / size / width", type: "—", desc: "Passed through. Placeholder defaults to common.selectSupplier." },
        ],
        usage: `<SupplierSelect size="sm" value={supplierId} onChange={setSupplierId} />

// Locked / read-only (edit drawer):
<SupplierSelect value={agreement.supplierId} disabled />

// Table cell — code-first, so a Supplier column scans down cleanly:
{supplierLabel(supplierRefs.get(po.supplierId)) ?? "—"}`,
        notes: "It resolves its OWN trigger label via useSupplierRefs on the single selected id, so a pre-set value (edit drawer, filter restored from a URL, ?supplier= lock) never flashes a raw UUID — every call site used to hand-roll that same IIFE. Don't re-add the filter id to a page's useSupplierRefs memo; that list is for table cells now. TWO label formats, deliberately: the picker (rows + trigger) leads with the NAME — a trigger is read on its own, right after clicking a row whose name led — while the exported supplierLabel() keeps CODE · Name for TABLE CELLS, where a column is scanned straight down and the fixed-width code is what makes that scan work. Import supplierLabel for cells; never re-inline either template. Rows are a Building2 glyph (the sidebar's own supplier icon, in a 32px slot matching UserAvatar size=\"sm\") + name over a muted code; the same glyph sits in the control via startElement, so the field reads as \"a supplier goes here\" empty and as the picked supplier once set.",
        Demo: demo.SupplierSelectDemo,
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
      {
        id: "batch-select",
        name: "BatchSelect",
        file: "src/components/BatchSelect.tsx",
        summary: "Batch/lot picker as a button → searchable modal (mirrors WarehouseSelect). Rows are a product thumbnail + product name over batch no · expiry · available qty; the trigger previews the picked lot.",
        props: [
          { name: "value / onChange", type: "string / (id: string) => void", desc: "Batch id (\"\" = none)." },
          { name: "onSelectItem", type: "(batch: Batch) => void", desc: "Hands back the full picked batch — product name + available qty without a second lookup." },
          { name: "warehouseId", type: "string", desc: "Scopes availability AND the in-stock filter to one warehouse (e.g. a transfer source). Empty = the caller's active warehouse." },
          { name: "onlyInStock", type: "boolean", desc: "Default TRUE — note searchBatches defaults it to false. See the notes." },
          { name: "productId", type: "string", desc: "Scopes results to one product's lots (the stocktake add-batch dialog pairs this with its own product filter)." },
          { name: "clearable", type: "boolean", desc: "Inline × that emits \"\". Default false — a required line field must not offer \"none\"; a filter must." },
          { name: "excludeIds", type: "readonly string[]", desc: "Hides already-picked lots (the selected value is never hidden)." },
          { name: "selectedLabel", type: "string", desc: "Trigger label when the picked lot isn't in the currently-loaded list." },
          { name: "placeholder / disabled / size / width", type: "—", desc: "Passed through. Placeholder defaults to transfers.pickBatch." },
        ],
        usage: `<BatchSelect
  warehouseId={from}
  value=""
  onSelectItem={appendLine}
  onlyInStock
  excludeIds={lines.map((l) => l.batchId)}
  disabled={!from}
  placeholder={t("transfers.addBatch")}
/>

// Ledger surfaces must see depleted lots too — their history is the point:
<BatchSelect value={batchId} onChange={setBatchId} onlyInStock={false} />`,
        notes: "Rows are deliberately two-line (a 32px product thumbnail, then product name on top and batch no · expiry · available qty below) so staff never has to memorize a batch number — that's the whole reason to reach for this over a bare <SearchableSelect loadOptions={searchBatches}>. The thumbnail rides on Batch.product_image_updated_at, enriched by the SAME SearchBatches join that fills product_name, so a row costs no extra round trip and a product with no picture (version 0) costs no request at all. Once picked, the trigger previews that lot's thumbnail in place of the Boxes glyph; the picked row is held in local state because close() clears the loaded list, and re-deriving the trigger from that list is what used to blank the label out on close. TWO gotchas. (1) onlyInStock defaults TRUE here while searchBatches defaults it FALSE, so a call site converted from the raw select silently loses depleted lots unless it passes onlyInStock={false} — wrong for a ledger filter (a spent batch still has movements) and for a positive ADJUSTMENT (that's how you correct an empty batch upward). (2) There is NO clear affordance, unlike SearchableSelect's Combobox.ClearTrigger — so it fits a required line field (Transfers), not a filter you need to reset to \"all\".",
        Demo: demo.BatchSelectDemo,
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
      {
        id: "product-picker-dialog",
        name: "ProductPickerDialog",
        file: "src/components/ProductPickerDialog.tsx",
        summary:
          "Multi-select product picker — searchable, server-paginated modal. For building a LIST of products (restock lines); <SearchableSelect loadOptions={searchProducts}> stays the single-value tool.",
        props: [
          { name: "open / onClose", type: "boolean / () => void", required: true, desc: "Controlled. onClose fires on Cancel / Esc / backdrop and does NOT commit." },
          { name: "selectedIds", type: "readonly string[]", required: true, desc: "Read at open time — the dialog opens pre-checked with these. Re-renders mid-edit do not clobber the draft." },
          { name: "onConfirm", type: "(ids: string[], byId: Map<string, Product>) => void", required: true, desc: "The full checked set + every Product loaded this session (so a newly-checked id always resolves to its Product, incl. `units`)." },
          { name: "title", type: "string", desc: "Defaults to t(\"productPicker.title\")." },
        ],
        usage: `<ProductPickerDialog
  open={pickerOpen}
  onClose={() => setPickerOpen(false)}
  selectedIds={lines.map((l) => l.productId)}
  onConfirm={applyPicked}
/>`,
        notes:
          "The check set IS the caller's list: reconcile on confirm (new ids → rows, dropped ids → removed, kept ids keep their edits). Stays mounted while closed (body-lock rule) but idle — the query is `enabled: open`. Its gallery demo is the one that performs a real request; server-paginated search can't be shown with fixtures.",
        Demo: demo.ProductPickerDialogDemo,
      },
    ],
  },
  {
    id: "data",
    labelKey: "data",
    icon: Table2,
    entries: [
      {
        id: "table-scroll",
        name: "TableScroll",
        file: "src/components/TableScroll.tsx",
        summary:
          "Bounded scroll box for a data table — caps its height, keeps the header stuck, and stops a wide table pushing the page.",
        props: [
          { name: "children", type: "ReactNode", required: true, desc: "The <Table.Root>. Give it `stickyHeader`." },
          { name: "maxH", type: "string", desc: "Viewport cap. Default TABLE_MAX_H (list page); use TABLE_MAX_H_NESTED inside a card/tab." },
          { name: "framed", type: "boolean", desc: "Default true: draws the border + surface. Pass false when already inside a Card." },
        ],
        usage: `<TableScroll>
  <Table.Root size="sm" stickyHeader>
    <Table.Header>…</Table.Header>
    <Table.Body>…</Table.Body>
  </Table.Root>
</TableScroll>
<Pagination … />`,
        notes:
          "Wraps Chakra's Table.ScrollArea (overflow:auto + max-width:100%), so the table scrolls on BOTH axes inside its own box and can never widen the page. Move bg/borderWidth/borderRadius OFF Table.Root or you get a double border. The header background is applied here — Chakra's default `line` variant gives headers none, and a transparent sticky header lets rows scroll through it.",
        Demo: demo.TableScrollDemo,
      },
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
        id: "chart-card",
        name: "ChartCard",
        file: "src/components/ChartCard.tsx",
        summary: "The frame every chart sits in: title, description, actions slot + loading / empty states.",
        props: [
          { name: "title", type: "string", required: true, desc: "Localized chart title. Names the single series, so no legend is needed." },
          { name: "description", type: "string", desc: "Muted sub-line under the title." },
          { name: "actions", type: "ReactNode", desc: "Right-aligned header slot (a filter, a toggle)." },
          { name: "height", type: "string", desc: 'Plot height. Default "240px".' },
          { name: "isLoading", type: "boolean", desc: "Centered spinner instead of the plot." },
          { name: "isEmpty", type: "boolean", desc: 'Centered t("common.noResults") instead of the plot.' },
        ],
        usage: `<ChartCard title={t("dashboard.trend.last7d")} isLoading={q.isLoading} isEmpty={rows.length === 0}>
  <TrendChart data={rows} xKey="day" money series={[{ dataKey: "revenue", label: t("…") }]} />
</ChartCard>`,
        notes: "Same card vocabulary as DashboardTile (bg.subtle + border + radius lg). Don't hand-roll a Box around a ResponsiveContainer.",
        Demo: demo.ChartCardDemo,
      },
      {
        id: "trend-chart",
        name: "TrendChart",
        file: "src/components/TrendChart.tsx",
        summary: "The app's line/area chart — Chakra-tokened axes, grid, crosshair and tooltip; follows light/dark.",
        props: [
          { name: "data", type: "Record<string, string | number>[]", required: true, desc: "One row per X point." },
          { name: "xKey", type: "string", required: true, desc: "Row key for the X axis (day string / bucket label)." },
          { name: "series", type: "TrendSeries[]", required: true, desc: "{ dataKey, label, colorIndex? }. One series → filled area; 2+ → lines + legend." },
          { name: "money", type: "boolean", desc: "Compact currency ticks on the axis, exact formatMoney in the tooltip." },
        ],
        usage: `<TrendChart
  data={trendData}
  xKey="day"
  money
  series={[{ dataKey: "revenue", label: t("analytics.metric.order.terjual"), colorIndex: 0 }]}
/>`,
        notes: "Colours come from CHART_SERIES in lib/chartTheme.ts — a fixed 4-slot order (blue, orange, teal, purple), never cycled, validated for colour-blind separation and contrast in both modes. Pin `colorIndex` so a metric keeps one colour across pages (revenue = slot 0). Never a second Y axis: two measures of different scale = two charts.",
        Demo: demo.TrendChartDemo,
      },
      {
        id: "barcode",
        name: "Barcode",
        file: "src/components/Barcode.tsx",
        summary: "CODE128 barcode as inline SVG — for product labels and anything scannable.",
        props: [
          { name: "value", type: "string", required: true, desc: "Data to encode. For products this is the SKU, since POS scans by exact-SKU match." },
          { name: "width", type: "number", desc: "Narrow-module width in px. Default 2 — higher is wider and easier to scan." },
          { name: "height", type: "number", desc: "Bar height in px. Default 60." },
          { name: "displayValue", type: "boolean", desc: "Print the value as text under the bars. Default true." },
          { name: "fontSize", type: "number", desc: "Size of that text in px. Default 14." },
          { name: "margin", type: "number", desc: "Quiet-zone margin in px. Default 10 — scanners need one, don't set it to 0." },
        ],
        usage: `<Barcode value={product.sku} height={50} fontSize={13} />`,
        notes:
          "Always black-on-white in BOTH themes: a barcode is an optical target, so inverting it in dark mode makes it unscannable and prints as a black rectangle. Encoding is jsbarcode, not a hand-rolled module table — a wrong table yields a symbol that prints perfectly and scans as a different string. CODE128 code set B only covers printable ASCII: check `isCode128Encodable` (lib/barcode.ts) before offering a Print action, since the backend's `printer.Code128Encodable` refuses the same inputs. An unencodable value renders a warning, never a fake symbol.",
        Demo: demo.BarcodeDemo,
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
      {
        id: "user-avatar",
        name: "UserAvatar",
        file: "src/components/UserAvatar.tsx",
        summary: "Profile picture with initials fallback; reads the THUMB rendition by default.",
        props: [
          { name: "userId", type: "string", required: true, desc: "Whose picture to load." },
          { name: "name", type: "string", required: true, desc: "Display name — drives the initials fallback and alt text." },
          { name: "version", type: "number", required: true, desc: "The user's avatarUpdatedAt (unix sec). 0 = no picture: skips the fetch entirely." },
          { name: "size", type: '"2xs" | "xs" | "sm" | "md" | "lg" | "xl" | "2xl"', desc: "Chakra Avatar size. Default \"sm\"." },
          { name: "full", type: "boolean", desc: "Load the ORIGINAL rendition instead of the thumbnail. Only for a deliberate full-size view." },
        ],
        usage: `<UserAvatar userId={user.id} name={displayName(user)} version={Number(user.avatarUpdatedAt)} size="xs" />`,
        notes:
          "Two-rendition HARD RULE: leave `full` off everywhere except a genuine full-size view — a table of 25 users on the original would pull megabytes. `version` doubles as the cache key, so a re-upload appears with no invalidate, and version=0 costs zero requests instead of fetching-then-discarding.",
        Demo: demo.UserAvatarDemo,
      },
      {
        id: "product-image",
        name: "ProductImage",
        file: "src/components/ProductImage.tsx",
        summary: "Product photo with a neutral box placeholder; reads the THUMB rendition by default.",
        props: [
          { name: "productId", type: "string", required: true, desc: "Whose picture to load." },
          { name: "name", type: "string", required: true, desc: "Display name — used as the alt text." },
          { name: "version", type: "number", required: true, desc: "The product's imageUpdatedAt (unix sec). 0 = no picture: skips the fetch entirely." },
          { name: "size", type: "number", desc: "Rendered square size in px. Default 40." },
          { name: "full", type: "boolean", desc: "Load the ORIGINAL rendition instead of the thumbnail. Only for a deliberate full-size view." },
          { name: "zoomable", type: "boolean", desc: "Clicking opens a lightbox with the ORIGINAL. Ignored when version === 0. The click is stopPropagation'd, so it is safe inside a clickable row." },
        ],
        usage: `<ProductImage productId={p.id} name={p.name} version={Number(p.imageUpdatedAt)} size={56} zoomable />`,
        notes:
          "Two-rendition HARD RULE: leave `full` off everywhere except a genuine full-size view (today only the product-detail picker). `zoomable` is the cheap way to offer the original — its lightbox fetches ORIGINAL only WHILE OPEN, so a list of 25 thumbnails never pulls the heavy bytes. The lightbox's Dialog.Root stays mounted and is driven by `open` (dialog-mount HARD RULE); only its content is lazyMount/unmountOnExit. In POS one product can occupy several search rows — one per sellable unit — and they all share one cache entry because `version` is the key. objectFit is `contain`, not `cover`: the thumb is already centre-cropped square, and cropping a product shot twice cuts the label off.",
        Demo: demo.ProductImageDemo,
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
        summary: "Grafana-style time-range picker: one button opening a popover with an optional which-date row, an absolute range and searchable quick ranges.",
        props: [
          { name: "value", type: "DateRange", required: true, desc: "{ preset, fromUnix, toUnix, customFrom?, customTo? } — from lib/dateRange." },
          { name: "onChange", type: "(next: DateRange) => void", required: true, desc: "Already resolved — send fromUnix/toUnix straight to the RPC." },
          { name: "size", type: '"xs" | "sm" | "md"', desc: "Control height. Default sm (toolbar size)." },
          { name: "fields", type: "DateFieldOption[]", desc: 'Which date columns the range can apply to, e.g. [{value:"created",label:"Created"}]. Omit on a surface with only one filterable date — the row then isn\'t rendered.' },
          { name: "field", type: "string", desc: 'The selected column. "" = Any date (the picker\'s own off state).' },
          { name: "onFieldChange", type: "(field: string) => void", desc: "Fires with the column value, or \"\" for Any date." },
        ],
        usage: `import { resolveRange, type DateRange } from "../lib/dateRange";

const [range, setRange] = useState<DateRange>(() => resolveRange("30d"));
<DateRangeFilter value={range} onChange={setRange} />
// then: useListSalesQuery({ fromUnix: BigInt(range.fromUnix), toUnix: BigInt(range.toUnix) })

// With a which-date row (list has several filterable dates):
const [dateField, setDateField] = useState("");   // "" = Any date
<DateRangeFilter
  value={range} onChange={setRange}
  fields={[{ value: "created", label: t("purchasing.dateCreated") },
           { value: "received", label: t("purchasing.dateReceived") }]}
  field={dateField} onFieldChange={setDateField}
/>
// then send NO bounds while the field is "":
//   dateField, fromUnix: dateField ? BigInt(range.fromUnix) : 0n, toUnix: ...`,
        notes: "The model lives in lib/dateRange.ts, not here: resolveRange (quick range → bounds, re-resolved against now), absoluteRange, rangeBounds, rangeLabel, parseAbsolute/formatAbsolute. Day-aligned ranges are [from, to) — the end is the start of the next day. Picking a quick range keeps its preset (re-resolved against now on every pick); typing bounds or using the calendar yields preset \"custom\", pinned to those instants. The inline calendar is the shared CalendarViews from components/DatePicker.tsx — reuse it for any new calendar surface, never re-implement the grids. The which-date row is a SegmentGroup, not an EnumSelect: a nested select popover inside this popover fights it for outside-click. Keep the field labels bare nouns (\"Created\", not \"By created date\") — they render as segments, and the trigger already reads \"Created · Last 30 days\".",
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
