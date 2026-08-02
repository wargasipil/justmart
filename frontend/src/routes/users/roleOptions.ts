import { useTranslation } from "react-i18next";

import { Role } from "../../gen/auth_iface/v1/policy_pb";
import { useBusinessMode } from "../../queries/settings";

// Mode-specific roles carry the mode that unlocks them; `undefined` = shared by
// every mode. Mirrors the backend's rolesByMode (service/user/roles.go) — the
// two must stay in sync, or the form offers a role the server then rejects.
const ROLE_OPTIONS: { value: Role; key: string; onlyIn?: "pharmacy" | "restaurant" }[] = [
  { value: Role.OWNER, key: "owner" },
  { value: Role.PHARMACIST, key: "pharmacist" }, // labeled "Admin" (manager tier, every mode)
  { value: Role.APOTEKER, key: "apoteker", onlyIn: "pharmacy" },
  { value: Role.WAITER, key: "waiter", onlyIn: "restaurant" },
  { value: Role.CASHIER, key: "cashier" },
];

// useRoleOptions returns the assignable roles for the current business mode:
// APOTEKER (Rx authority) is pharmacy-only and WAITER (floor service) is
// restaurant-only — kept out of the other modes to stop roles mixing (mirrors the
// backend rolesByMode). `include` forces a role to remain even when filtered, so
// an existing out-of-mode user's current role still renders in their row instead
// of showing blank.
export function useRoleOptions(include?: Role): { value: string; label: string }[] {
  const { t } = useTranslation();
  const { isPharmacy, isRestaurant } = useBusinessMode();
  const available = { pharmacy: isPharmacy, restaurant: isRestaurant };
  return ROLE_OPTIONS.filter(
    (o) => !o.onlyIn || available[o.onlyIn] || o.value === include,
  ).map((o) => ({ value: String(o.value), label: t(`dashboard.roles.${o.key}`) }));
}
