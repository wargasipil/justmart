import { useEffect } from "react";
import {
  Badge,
  Box,
  Code,
  Flex,
  HStack,
  Heading,
  Spacer,
  Stack,
  Table,
  Text,
} from "@chakra-ui/react";
import { useTranslation } from "react-i18next";
import { Navigate, useLocation, useNavigate, useParams } from "react-router-dom";

import PageHeader from "../../components/PageHeader";
import {
  DEFAULT_GROUP,
  GROUPS,
  TOTAL_ENTRIES,
  findGroup,
  type ComponentEntry,
  type ComponentGroup,
} from "./registry";

// Dev-only component gallery at /components — the curated catalog of shared
// components (see registry.ts for what earns an entry). Registered from
// main.tsx behind `import.meta.env.DEV`, so it never ships to a shop.
//
// Layout: a grouped left rail (group = the context you reach for a component
// in) + one section per component with a live preview, its props, and a
// copy-pasteable call site.
export default function Components() {
  const { t } = useTranslation();
  const { group: groupId } = useParams();
  const location = useLocation();
  const group = findGroup(groupId);

  // Deep links carry the component anchor (/components/forms#money-input);
  // scroll to it once the group's sections are on the page.
  useEffect(() => {
    if (!location.hash) return;
    const el = document.getElementById(location.hash.slice(1));
    if (el) el.scrollIntoView({ behavior: "smooth", block: "start" });
  }, [location.hash, groupId]);

  if (!group) return <Navigate to={`/components/${DEFAULT_GROUP}`} replace />;

  return (
    <Box>
      <PageHeader
        breadcrumbs={[
          { label: t("dev.components.title") },
          { label: t(`dev.components.groups.${group.labelKey}`) },
        ]}
        title={t("dev.components.title")}
        description={t("dev.components.description", { total: TOTAL_ENTRIES })}
        actions={<Badge colorPalette="orange">{t("dev.components.badge")}</Badge>}
      />

      <Flex gap={6} align="flex-start">
        <GroupNav activeGroup={group} />
        <Stack gap={8} flex="1" minW={0}>
          {group.entries.map((entry) => (
            <EntrySection key={entry.id} entry={entry} />
          ))}
        </Stack>
      </Flex>
    </Box>
  );
}

// Grouped rail: every group with its component list. The active group is
// expanded (its children scroll the page); the others navigate on click.
function GroupNav({ activeGroup }: { activeGroup: ComponentGroup }) {
  const { t } = useTranslation();
  const navigate = useNavigate();

  const jump = (group: ComponentGroup, entryId: string) => {
    if (group.id === activeGroup.id) {
      document.getElementById(entryId)?.scrollIntoView({ behavior: "smooth", block: "start" });
      return;
    }
    navigate(`/components/${group.id}#${entryId}`);
  };

  return (
    <Stack
      as="nav"
      gap={4}
      width="230px"
      flexShrink={0}
      position="sticky"
      top="72px"
      maxH="calc(100vh - 96px)"
      overflowY="auto"
      display={{ base: "none", lg: "flex" }}
      pe={2}
    >
      {GROUPS.map((group) => {
        const Icon = group.icon;
        const active = group.id === activeGroup.id;
        return (
          <Stack key={group.id} gap={1}>
            <HStack
              as="button"
              gap={2}
              px={2}
              py={1}
              borderRadius="md"
              color={active ? "colorPalette.solid" : "fg"}
              colorPalette="blue"
              _hover={{ bg: "bg.muted" }}
              cursor="pointer"
              onClick={() => navigate(`/components/${group.id}`)}
            >
              <Icon size={16} />
              <Text fontSize="sm" fontWeight="medium">
                {t(`dev.components.groups.${group.labelKey}`)}
              </Text>
              <Spacer />
              <Text fontSize="xs" color="fg.muted">
                {group.entries.length}
              </Text>
            </HStack>
            <Stack gap={0} ps={3} borderLeftWidth="1px">
              {group.entries.map((entry) => (
                <Text
                  key={entry.id}
                  as="button"
                  textAlign="start"
                  px={2}
                  py={1}
                  borderRadius="md"
                  fontSize="xs"
                  fontFamily="mono"
                  color={active ? "fg.muted" : "fg.subtle"}
                  _hover={{ bg: "bg.muted", color: "fg" }}
                  cursor="pointer"
                  onClick={() => jump(group, entry.id)}
                >
                  {entry.name}
                </Text>
              ))}
            </Stack>
          </Stack>
        );
      })}
    </Stack>
  );
}

// One component: header (name + source path), summary, live preview, props
// table, usage snippet, optional gotcha.
function EntrySection({ entry }: { entry: ComponentEntry }) {
  const { t } = useTranslation();
  const Demo = entry.Demo;
  return (
    <Stack
      id={entry.id}
      gap={4}
      scrollMarginTop="72px"
      borderWidth="1px"
      borderRadius="lg"
      bg="bg.subtle"
      p={5}
    >
      <Stack gap={1}>
        <HStack gap={3} flexWrap="wrap">
          <Heading size="md" fontFamily="mono">
            {entry.name}
          </Heading>
          <Code size="sm" color="fg.muted">
            {entry.file}
          </Code>
        </HStack>
        <Text fontSize="sm" color="fg.muted">
          {entry.summary}
        </Text>
      </Stack>

      <Stack gap={2}>
        <SectionLabel>{t("dev.components.preview")}</SectionLabel>
        <Box borderWidth="1px" borderRadius="md" bg="bg" p={4}>
          <Demo />
        </Box>
      </Stack>

      <Stack gap={2}>
        <SectionLabel>{t("dev.components.props")}</SectionLabel>
        <Box overflowX="auto">
          <Table.Root size="sm" variant="line">
            <Table.Header>
              <Table.Row>
                <Table.ColumnHeader>{t("dev.components.propName")}</Table.ColumnHeader>
                <Table.ColumnHeader>{t("dev.components.propType")}</Table.ColumnHeader>
                <Table.ColumnHeader>{t("dev.components.propDesc")}</Table.ColumnHeader>
              </Table.Row>
            </Table.Header>
            <Table.Body>
              {entry.props.map((p) => (
                <Table.Row key={p.name}>
                  <Table.Cell whiteSpace="nowrap">
                    <HStack gap={2}>
                      <Code size="sm">{p.name}</Code>
                      {p.required && (
                        <Badge size="sm" colorPalette="blue">
                          {t("dev.components.required")}
                        </Badge>
                      )}
                    </HStack>
                  </Table.Cell>
                  <Table.Cell color="fg.muted" fontFamily="mono" fontSize="xs">
                    {p.type}
                  </Table.Cell>
                  <Table.Cell fontSize="xs">{p.desc}</Table.Cell>
                </Table.Row>
              ))}
            </Table.Body>
          </Table.Root>
        </Box>
      </Stack>

      <Stack gap={2}>
        <SectionLabel>{t("dev.components.usage")}</SectionLabel>
        <Code
          display="block"
          whiteSpace="pre"
          overflowX="auto"
          p={3}
          borderRadius="md"
          fontSize="xs"
        >
          {entry.usage}
        </Code>
      </Stack>

      {entry.notes && (
        <Stack gap={2}>
          <SectionLabel>{t("dev.components.notes")}</SectionLabel>
          <Box
            borderStartWidth="3px"
            borderStartColor="orange.400"
            bg="bg"
            borderRadius="md"
            px={3}
            py={2}
          >
            <Text fontSize="xs">{entry.notes}</Text>
          </Box>
        </Stack>
      )}
    </Stack>
  );
}

function SectionLabel({ children }: { children: string }) {
  return (
    <Text
      fontSize="xs"
      color="fg.muted"
      fontWeight="medium"
      textTransform="uppercase"
      letterSpacing="wider"
    >
      {children}
    </Text>
  );
}
