import {
  ArrowLeftRight,
  BarChart3,
  Blocks,
  Boxes,
  Building2,
  ClipboardList,
  FileText,
  Grid2X2,
  Handshake,
  LayoutDashboard,
  Package,
  Pill,
  Receipt,
  Repeat,
  Settings as SettingsIcon,
  ShoppingBag,
  ShoppingCart,
  Truck,
  UserRound,
  Users as UsersIcon,
  UtensilsCrossed,
  Warehouse as WarehouseIcon,
} from "lucide-react";

import { Role } from "../gen/auth_iface/v1/policy_pb";

// The nav catalog: WHAT the rail contains, kept apart from Sidebar.tsx, which is
// HOW it renders (collapse states, groups, active styling). It is a pure
// data structure plus its types, so it reads as a list you can scan and edit —
// which is what people actually come here to do — instead of being buried above
// 180 lines of JSX.
// A nav entry restricted to one business mode. Replaces the old boolean
// `pharmacyOnly` — with three modes a per-mode boolean per entry stops scaling,
// and a missing one silently leaks the entry into every other mode. Same
// vocabulary as `onlyIn` in routes/users/roleOptions.ts.
export type ModeOnly = "pharmacy" | "restaurant";

export type NavLeaf = {
  to: string;
  label: string;
  icon: typeof Package;
  roles?: Role[];
  onlyIn?: ModeOnly; // hidden unless the shop runs in this mode
  devOnly?: boolean; // dev builds only (the /components gallery)
};

export type NavGroup = {
  kind: "group";
  label: string;
  icon: typeof Package;
  roles?: Role[];
  onlyIn?: ModeOnly;
  children: NavLeaf[];
};

export type NavEntry = NavLeaf | NavGroup;

export function isGroup(e: NavEntry): e is NavGroup {
  return "kind" in e && e.kind === "group";
}

export function buildItems(
  t: (k: string) => string,
  mode: { isPharmacy: boolean; isRestaurant: boolean },
): NavEntry[] {
  const { isPharmacy, isRestaurant } = mode;
  // The catalog noun + mark change per mode: Obat/Pill (pharmacy), Menu/Utensils
  // (restaurant), Produk/Package (retail).
  const catalogLabel = isPharmacy
    ? t("nav.medicines")
    : isRestaurant
      ? t("nav.menu")
      : t("nav.products");
  const catalogIcon = isPharmacy ? Pill : isRestaurant ? UtensilsCrossed : Package;
  return [
    { to: "/", label: t("nav.dashboard"), icon: LayoutDashboard },
    {
      to: "/analytics",
      label: t("nav.analytics"),
      icon: BarChart3,
      roles: [Role.OWNER, Role.PHARMACIST],
    },
    {
      kind: "group",
      label: t("nav.order"),
      icon: ShoppingBag,
      children: [
        // Restaurant: the floor view (open tables) is where an order starts, so
        // it sits above the till. Waiters have both; only the till roles can
        // settle a bill.
        {
          to: "/tables",
          label: t("nav.tables"),
          icon: Grid2X2,
          onlyIn: "restaurant",
        },
        { to: "/pos", label: t("nav.pos"), icon: ShoppingCart },
        {
          to: "/orders",
          label: t("nav.orders"),
          icon: Receipt,
          roles: [Role.OWNER, Role.PHARMACIST, Role.CASHIER, Role.APOTEKER, Role.WAITER],
        },
      ],
    },
    {
      to: "/products",
      label: catalogLabel,
      icon: catalogIcon,
      roles: [Role.OWNER, Role.PHARMACIST],
    },
    {
      kind: "group",
      label: t("nav.inventory"),
      icon: Package,
      roles: [Role.OWNER, Role.PHARMACIST],
      children: [
        { to: "/purchasing", label: t("nav.purchasing"), icon: Truck },
        { to: "/inventory/suppliers", label: t("inventory.tabs.suppliers"), icon: Building2 },
        { to: "/inventory/price-agreements", label: t("nav.priceAgreements"), icon: Handshake },
        { to: "/inventory/batches", label: t("inventory.tabs.batches"), icon: Boxes },
        { to: "/inventory/movements", label: t("inventory.tabs.movements"), icon: ArrowLeftRight },
        { to: "/inventory/stocktake", label: t("inventory.tabs.stocktake"), icon: ClipboardList },
        { to: "/inventory/transfers", label: t("inventory.tabs.transfers"), icon: Repeat },
      ],
    },
    {
      // Resep (prescriptions) — pharmacy mode. Visible to the Rx authority
      // (OWNER + PHARMACIST + APOTEKER). Phase 5 will additionally gate this on
      // the active business mode (hidden in retail mode).
      to: "/prescriptions",
      label: t("nav.prescriptions"),
      icon: FileText,
      roles: [Role.OWNER, Role.PHARMACIST, Role.APOTEKER],
      onlyIn: "pharmacy",
    },
    {
      to: "/customers",
      label: t("nav.customers"),
      icon: UserRound,
    },
    {
      // Self-scoped performance view for the till roles (own sales over time).
      // OWNER/PHARMACIST get full Analytics instead, so they don't see this.
      to: "/my-performance",
      label: t("nav.myPerformance"),
      icon: BarChart3,
      roles: [Role.CASHIER, Role.APOTEKER, Role.WAITER],
    },
    { to: "/warehouses", label: t("nav.warehouses"), icon: WarehouseIcon, roles: [Role.OWNER] },
    { to: "/users", label: t("nav.users"), icon: UsersIcon, roles: [Role.OWNER] },
    { to: "/settings", label: t("nav.settings"), icon: SettingsIcon, roles: [Role.OWNER] },
    // Dev tool: the curated shared-component gallery. Stripped from the rail
    // (and from the router) in a production build.
    { to: "/components", label: t("nav.components"), icon: Blocks, devOnly: true },
  ];
}

