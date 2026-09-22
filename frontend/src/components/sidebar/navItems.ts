import {
  ArrowLeftRight,
  BarChart3,
  Blocks,
  Boxes,
  Building2,
  ClipboardList,
  Factory,
  FileText,
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
  Warehouse as WarehouseIcon,
} from "lucide-react";

import { Role } from "../../gen/auth_iface/v1/policy_pb";

// The nav model: what the rail and the phone drawer list, and for whom.
// Filtering by role / mode / dev build happens in SidebarNav.

export type NavLeaf = {
  to: string;
  label: string;
  icon: typeof Package;
  roles?: Role[];
  pharmacyOnly?: boolean; // hidden unless the shop is in pharmacy mode
  devOnly?: boolean; // dev builds only (the /components gallery)
};

export type NavGroup = {
  kind: "group";
  label: string;
  icon: typeof Package;
  roles?: Role[];
  children: NavLeaf[];
};

export type NavEntry = NavLeaf | NavGroup;

export function isGroup(e: NavEntry): e is NavGroup {
  return "kind" in e && e.kind === "group";
}

export function buildItems(t: (k: string) => string, isPharmacy: boolean): NavEntry[] {
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
        { to: "/pos", label: t("nav.pos"), icon: ShoppingCart },
        {
          to: "/orders",
          label: t("nav.orders"),
          icon: Receipt,
          roles: [Role.OWNER, Role.PHARMACIST, Role.CASHIER, Role.APOTEKER],
        },
      ],
    },
    {
      to: "/products",
      // In pharmacy mode the catalog is "Obat" (medicines); in retail it's "Produk".
      label: isPharmacy ? t("nav.medicines") : t("nav.products"),
      icon: isPharmacy ? Pill : Package,
      // All four roles: the till gets a read-only, cost-free view of the same
      // pages so a cashier can look up a price or a stock level.
      roles: [Role.OWNER, Role.PHARMACIST, Role.CASHIER, Role.APOTEKER],
    },
    {
      kind: "group",
      label: t("nav.inventory"),
      icon: Package,
      roles: [Role.OWNER, Role.PHARMACIST],
      children: [
        { to: "/purchasing", label: t("nav.purchasing"), icon: Truck },
        { to: "/inventory/suppliers", label: t("inventory.tabs.suppliers"), icon: Building2 },
        // Pabrik (manufacturers): the maker, listed next to the seller.
        { to: "/inventory/manufacturers", label: t("inventory.manufacturers.title"), icon: Factory },
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
      pharmacyOnly: true,
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
      roles: [Role.CASHIER, Role.APOTEKER],
    },
    { to: "/warehouses", label: t("nav.warehouses"), icon: WarehouseIcon, roles: [Role.OWNER] },
    { to: "/users", label: t("nav.users"), icon: UsersIcon, roles: [Role.OWNER] },
    { to: "/settings", label: t("nav.settings"), icon: SettingsIcon, roles: [Role.OWNER] },
    // Dev tool: the curated shared-component gallery. Stripped from the rail
    // (and from the router) in a production build.
    { to: "/components", label: t("nav.components"), icon: Blocks, devOnly: true },
  ];
}


/**
 * The nav leaf the given path belongs to: the longest `to` that is the path
 * itself or a parent segment of it (`/products/abc` → `/products`). `/` only
 * matches exactly, or every path would belong to it. Groups are searched
 * through their children. `undefined` when no entry owns the path.
 */
export function findNavLeaf(entries: NavEntry[], pathname: string): NavLeaf | undefined {
  const leaves = entries.flatMap((e) => (isGroup(e) ? e.children : [e]));
  let best: NavLeaf | undefined;
  for (const leaf of leaves) {
    const hit =
      leaf.to === "/" ? pathname === "/" : pathname === leaf.to || pathname.startsWith(`${leaf.to}/`);
    if (hit && (!best || leaf.to.length > best.to.length)) best = leaf;
  }
  return best;
}
