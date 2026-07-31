import { HStack, Stack, Text } from "@chakra-ui/react";
import { useTranslation } from "react-i18next";

import type { UserRef } from "../gen/user_iface/v1/users_pb";
import { displayName, roleKey } from "../lib/roles";
import { searchSellingUsers } from "../queries/users";
import SearchableSelect from "./SearchableSelect";
import UserAvatar from "./UserAvatar";

// Cashier-scope picker for OWNER/Admin order-history + analytics filters. Wraps
// the shared <SearchableSelect> in async mode (HARD RULE for dynamic option
// sources). Empty value = "all cashiers" (cleared via the built-in clear
// trigger). Only render this for OWNER/PHARMACIST — cashiers are forced to self
// server-side and never see the picker.
//
// Options come from `searchSellingUsers`, so the list is exactly the people who
// appear in the data being filtered: no deactivated ex-staff, and nobody whose
// selection could only ever produce an empty table.
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
      loadOptions={searchSellingUsers}
      // Trigger stays one line — the rich layout below is the open list only.
      itemToString={(u) => displayName(u)}
      itemToValue={(u) => u.id}
      renderItem={(u) => <CashierOption user={u} />}
      selectedLabel={selectedLabel}
      placeholder={t("filters.cashierAll")}
      size="sm"
      width="220px"
    />
  );
}

// One dropdown row: the same identity cell the Users management table renders
// (picture + name over a muted second line), with the ROLE on that second line
// instead of the email. Role is what a filter needs to disambiguate — it tells
// an owner who also rings up sales apart from an actual cashier, which a bare
// name cannot; the email carries no meaning at the moment you pick someone.
function CashierOption({ user }: { user: UserRef }) {
  const { t } = useTranslation();
  return (
    <HStack gap={3} flex="1" minW={0}>
      <UserAvatar
        userId={user.id}
        name={displayName(user)}
        // 0 = no picture, which skips the fetch and renders initials.
        version={Number(user.avatarUpdatedAt)}
      />
      <Stack gap={0} flex="1" minW={0}>
        <Text fontWeight="medium" truncate>
          {user.name || user.email}
        </Text>
        <Text fontSize="xs" color="fg.muted" truncate>
          {t(`dashboard.roles.${roleKey(user.role)}`)}
        </Text>
      </Stack>
    </HStack>
  );
}
