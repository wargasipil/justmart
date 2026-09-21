import { Role } from "../gen/auth_iface/v1/policy_pb";

// roleKey maps the Role enum to the i18n suffix under `dashboard.roles.*`.
// Shared so the Dashboard header and the TopBar user menu can't drift apart.
export function roleKey(role: Role): string {
  switch (role) {
    case Role.OWNER:
      return "owner";
    case Role.PHARMACIST:
      return "pharmacist";
    case Role.CASHIER:
      return "cashier";
    case Role.APOTEKER:
      return "apoteker";
    default:
      return "unknown";
  }
}

// canSeeCost mirrors the backend's common.CanSeeCost: purchase cost (what the
// shop paid, and who it paid) is readable by every role — the catalog is fully
// readable by the till. Kept as a named predicate rather than inlined `true` so
// re-narrowing it stays one edit on each side; the backend redactors still run
// on every catalog read, so flipping this alone would leave the fields arriving
// as zeros. Change one side, change both.
//
// It is NOT the write gate — see canManageProducts.
export function canSeeCost(role: Role | undefined): boolean {
  return (
    role === Role.OWNER ||
    role === Role.PHARMACIST ||
    role === Role.CASHIER ||
    role === Role.APOTEKER
  );
}

// canManageProducts gates the catalog WRITE surfaces: create / import / edit /
// archive, the product image upload, and the price-tier + discount mutations.
// Those RPCs are OWNER+PHARMACIST in their proto allowed_roles, so rendering
// the controls for anyone else only produces a PermissionDenied toast.
//
// This used to be conflated with canSeeCost — one boolean hid both the cost
// figures and the buttons. Widening cost visibility to the till split them:
// cost is now readable by everyone, writing the catalog still is not.
export function canManageProducts(role: Role | undefined): boolean {
  return role === Role.OWNER || role === Role.PHARMACIST;
}

// displayName prefers the user's name and falls back to the email's local part
// ("pdcai64@gmail.com" -> "pdcai64") so the avatar never derives initials from
// the mail domain.
export function displayName(user: { name?: string; email?: string }): string {
  const name = user.name?.trim();
  if (name) return name;
  const email = user.email?.trim() ?? "";
  return email.split("@")[0] || "";
}

// initials returns 1-2 uppercase letters for the avatar fallback. Chakra's
// built-in `name` derivation yields a single letter for one-word names, so we
// take the first two characters in that case instead.
export function initials(label: string): string {
  const parts = label.trim().split(/\s+/).filter(Boolean);
  if (parts.length === 0) return "?";
  if (parts.length === 1) return parts[0].slice(0, 2).toUpperCase();
  return `${parts[0][0]}${parts[parts.length - 1][0]}`.toUpperCase();
}
