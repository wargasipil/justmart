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
// shop paid, and who it paid) is manager information. The till roles read the
// catalog — POS needs it, and the Products list/detail pages are open to them
// read-only — and the backend blanks the cost fields on those responses, so a
// page must not render cost cells (they would all be 0/"—") or fire the
// manager-only RPCs that carry it.
export function canSeeCost(role: Role | undefined): boolean {
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
