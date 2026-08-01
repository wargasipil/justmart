import { Box, HStack, Heading, Stack, Text } from "@chakra-ui/react";
import { useTranslation } from "react-i18next";
import type { ReactNode } from "react";

import EnumSelect from "../../components/EnumSelect";
import WarehouseSelect from "../../components/WarehouseSelect";
import {
  searchMyWarehouses,
  useMyWarehousesQuery,
  useSetDefaultWarehouseMutation,
} from "../../queries/warehouses";
import { usePreferencesStore, type Locale, type Theme } from "../../stores/preferences";

const LOCALES: Locale[] = ["id", "en"];
const THEMES: Theme[] = ["light", "dark"];

// Card vocabulary shared with DashboardTile / ChartCard: bg.subtle + hairline
// border + radius lg. Lives here rather than in components/ because it is
// layout for this one page — a shared component would owe the gallery a
// registry entry.
export function Section({
  title,
  help,
  children,
}: {
  title: string;
  help?: string;
  children: ReactNode;
}) {
  return (
    <Box borderWidth="1px" borderRadius="lg" bg="bg.subtle" p={5}>
      <Stack gap={1} mb={4}>
        <Heading size="sm">{title}</Heading>
        {help && (
          <Text fontSize="xs" color="fg.muted">
            {help}
          </Text>
        )}
      </Stack>
      {children}
    </Box>
  );
}

// Language + theme are the same two preferences the TopBar avatar menu flips;
// both read and write the one persisted store, so the two surfaces stay in
// sync without either owning the state.
export function PreferencesSection() {
  const { t } = useTranslation();
  return (
    <Section title={t("profile.preferences")} help={t("profile.preferencesHelp")}>
      <Stack gap={4}>
        <LanguageRow />
        <ThemeRow />
      </Stack>
    </Section>
  );
}

function LanguageRow() {
  const { t, i18n } = useTranslation();
  const locale = usePreferencesStore((s) => s.locale);
  const setLocale = usePreferencesStore((s) => s.setLocale);

  return (
    <PrefRow label={t("nav.language")}>
      <EnumSelect
        size="sm"
        width="180px"
        value={locale}
        onChange={(v) => {
          const next = v as Locale;
          setLocale(next);
          void i18n.changeLanguage(next);
        }}
        items={LOCALES}
        // Endonyms: a language is named in its own language, so these are not
        // translated (same call as the TopBar menu).
        itemToString={(l) => (l === "id" ? "Indonesia" : "English")}
        itemToValue={(l) => l}
      />
    </PrefRow>
  );
}

function ThemeRow() {
  const { t } = useTranslation();
  const theme = usePreferencesStore((s) => s.theme);
  const setTheme = usePreferencesStore((s) => s.setTheme);

  return (
    <PrefRow label={t("nav.theme")}>
      <EnumSelect
        size="sm"
        width="180px"
        value={theme}
        onChange={(v) => setTheme(v as Theme)}
        items={THEMES}
        itemToString={(v) => t(v === "dark" ? "nav.themeDark" : "nav.themeLight")}
        itemToValue={(v) => v}
      />
    </PrefRow>
  );
}

function PrefRow({ label, children }: { label: string; children: ReactNode }) {
  return (
    <HStack justify="space-between" gap={4} wrap="wrap">
      <Text fontSize="sm">{label}</Text>
      {children}
    </HStack>
  );
}

/**
 * Whether the default-warehouse section has anything to offer.
 *
 * Lifted out of the section itself because the Profile page renders it behind
 * a tab, and the TRIGGER has to disappear along with the panel — a tab that
 * opens onto nothing is worse than no tab at all. React Query dedupes this
 * with the call inside `DefaultWarehouseSection`, so it costs no extra fetch.
 */
export function useHasMultipleWarehouses() {
  const q = useMyWarehousesQuery();
  return (q.data?.warehouses.length ?? 0) >= 2;
}

// Which warehouse the user lands in when no explicit choice is persisted.
// Hidden for single-warehouse users — there is nothing to pick.
export function DefaultWarehouseSection() {
  const { t } = useTranslation();
  const myWarehousesQ = useMyWarehousesQuery();
  const setDefault = useSetDefaultWarehouseMutation();

  const data = myWarehousesQ.data;
  if (!data || data.warehouses.length < 2) return null;

  const current = data.memberships.find((m) => m.isDefault)?.warehouseId ?? "";
  const selected = data.warehouses.find((w) => w.id === current);

  return (
    <Section title={t("profile.defaultWarehouse")} help={t("profile.defaultWarehouseHelp")}>
      <WarehouseSelect
        size="sm"
        width="260px"
        value={current}
        onChange={(v) => setDefault.mutate({ warehouseId: v })}
        loadOptions={searchMyWarehouses}
        selectedLabel={selected ? `${selected.code} · ${selected.name}` : undefined}
      />
    </Section>
  );
}
