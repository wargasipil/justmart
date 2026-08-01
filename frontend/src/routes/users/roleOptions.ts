import { useTranslation } from "react-i18next";

import { Role } from "../../gen/auth_iface/v1/policy_pb";
import { useBusinessMode } from "../../queries/settings";

const ROLE_OPTIONS: { value: Role; key: string }[] = [
  { value: Role.OWNER, key: "owner" },
  { value: Role.PHARMACIST, key: "pharmacist" }, // labeled "Admin" (manager tier, both modes)
  { value: Role.APOTEKER, key: "apoteker" }, // pharmacy-only
  { value: Role.CASHIER, key: "cashier" },
];

// useRoleOptions returns the assignable roles for the current business mode:
// APOTEKER (the Rx-authority role) is pharmacy-only — kept out of retail to stop
// roles mixing across modes (mirrors the backend rolesByMode). `include` forces a
// role to remain even when filtered, so an existing out-of-mode user's current
// role still renders in their row instead of showing blank.
export function useRoleOptions(include?: Role): { value: string; label: string }[] {
  const { t } = useTranslation();
  const { isPharmacy } = useBusinessMode();
  return ROLE_OPTIONS.filter(
    (o) => o.value !== Role.APOTEKER || isPharmacy || o.value === include,
  ).map((o) => ({ value: String(o.value), label: t(`dashboard.roles.${o.key}`) }));
}
