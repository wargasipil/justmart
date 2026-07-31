import { Badge, HStack, IconButton, Menu, Portal, Text } from "@chakra-ui/react";
import { ArrowUpCircle, Bell, Settings as SettingsIcon } from "lucide-react";
import { useTranslation } from "react-i18next";
import { useNavigate } from "react-router-dom";

import { useLowStockQuery } from "../../queries/products";
import { useUpdateInfoQuery } from "../../queries/updates";

// LowStockBell polls ProductService.ListLowStock (active-warehouse-scoped) and
// renders a bell with a count badge + dropdown of low-stock products. Click an
// item → opens its detail page. OWNER also sees a footer link to /settings.
export default function LowStockBell({ isOwner }: { isOwner: boolean }) {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const q = useLowStockQuery();
  const meds = q.data?.products ?? [];
  const total = q.data?.total ?? 0;
  const threshold = q.data?.threshold ?? 0;

  // Update notice (OWNER only — the CheckUpdate RPC is OWNER-gated).
  const updQ = useUpdateInfoQuery(isOwner);
  const updateAvailable = isOwner && (updQ.data?.updateAvailable ?? false);
  const latest = updQ.data?.latestVersion ?? "";
  const badgeCount = total + (updateAvailable ? 1 : 0);

  return (
    <Menu.Root>
      <Menu.Trigger asChild>
        <IconButton
          aria-label={t("notifications.bellAria")}
          variant="ghost"
          size="sm"
          position="relative"
        >
          <Bell size={18} />
          {badgeCount > 0 && (
            <Badge
              position="absolute"
              top="2px"
              right="2px"
              colorPalette="red"
              minW="16px"
              h="16px"
              borderRadius="full"
              display="flex"
              alignItems="center"
              justifyContent="center"
              fontSize="9px"
              px={1}
            >
              {badgeCount > 99 ? "99+" : badgeCount}
            </Badge>
          )}
        </IconButton>
      </Menu.Trigger>
      <Portal>
        <Menu.Positioner>
          <Menu.Content minW="280px" maxH="360px" overflowY="auto">
            {updateAvailable && (
              <>
                <Menu.Item value="update" onClick={() => navigate("/settings/updates")}>
                  <HStack gap={2}>
                    <ArrowUpCircle size={14} />
                    <Text fontSize="sm" fontWeight="medium">
                      {t("notifications.updateAvailable", { version: latest })}
                    </Text>
                  </HStack>
                </Menu.Item>
                <Menu.Separator />
              </>
            )}
            <Menu.Item value="header" disabled>
              <Text fontSize="sm" fontWeight="medium">
                {t("notifications.lowStockTitle", { count: total })}
              </Text>
            </Menu.Item>
            <Menu.Separator />
            {meds.length === 0 ? (
              <Menu.Item value="empty" disabled>
                <Text fontSize="sm" color="fg.muted">
                  {t("notifications.empty")}
                </Text>
              </Menu.Item>
            ) : (
              meds.map((m) => (
                <Menu.Item
                  key={m.id}
                  value={m.id}
                  onClick={() => navigate(`/products/${m.id}`)}
                >
                  <HStack justify="space-between" w="100%" gap={2}>
                    <Text fontSize="sm" truncate>
                      {m.name}
                    </Text>
                    <Text fontSize="xs" color="fg.muted" whiteSpace="nowrap">
                      {m.readyStock.toString()} / {threshold}
                    </Text>
                  </HStack>
                </Menu.Item>
              ))
            )}
            {/* The list is one page now that ListLowStock is paginated. Say so
                rather than letting the dropdown look like the whole set — the
                header count above is the real total. */}
            {total > meds.length && (
              <Menu.Item value="more" disabled>
                <Text fontSize="xs" color="fg.muted">
                  {t("common.pagination.showing", {
                    from: 1,
                    to: meds.length,
                    total,
                  })}
                </Text>
              </Menu.Item>
            )}
            {isOwner && (
              <>
                <Menu.Separator />
                <Menu.Item value="settings" onClick={() => navigate("/settings")}>
                  <HStack gap={2}>
                    <SettingsIcon size={14} />
                    <Text fontSize="sm">{t("notifications.viewSettings")}</Text>
                  </HStack>
                </Menu.Item>
              </>
            )}
          </Menu.Content>
        </Menu.Positioner>
      </Portal>
    </Menu.Root>
  );
}
