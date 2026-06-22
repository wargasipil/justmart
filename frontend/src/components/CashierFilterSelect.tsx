import { useTranslation } from "react-i18next";

import type { UserRef } from "../gen/user_iface/v1/users_pb";
import { searchUsers } from "../queries/users";
import SearchableSelect from "./SearchableSelect";

// Cashier-scope picker for OWNER/Admin order-history + analytics filters. Wraps
// the shared <SearchableSelect> in async mode over searchUsers (HARD RULE for
// dynamic option sources). Empty value = "all cashiers" (cleared via the built-in
// clear trigger). Only render this for OWNER/PHARMACIST — cashiers are forced to
// self server-side and never see the picker.
export default function CashierFilterSelect({
  value,
  onChange,
  selectedLabel,
}: {
  value: string;
  onChange: (value: string) => void;
  selectedLabel?: string;
}) {
  const { t } = useTranslation();
  return (
    <SearchableSelect<UserRef>
      value={value || null}
      onChange={onChange}
      loadOptions={searchUsers}
      itemToString={(u) => u.name || u.email}
      itemToValue={(u) => u.id}
      selectedLabel={selectedLabel}
      placeholder={t("filters.cashierAll")}
      size="sm"
      width="220px"
    />
  );
}
