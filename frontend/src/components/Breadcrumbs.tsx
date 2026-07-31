import { Breadcrumb, Text } from "@chakra-ui/react";
import { ChevronRight } from "lucide-react";
import { Fragment } from "react";
import { Link as RouterLink } from "react-router-dom";

import { useBreadcrumbs } from "../lib/breadcrumbs";

/**
 * The app's "you are here" trail, rendered once in the TopBar — never per page.
 * The trail comes from the URL via the registry in lib/breadcrumbs.ts; a detail
 * page only contributes its entity name through useCrumbLabel().
 *
 * Narrow viewports collapse to the current crumb alone (the TopBar is 56px and
 * the right-hand controls must not be pushed off screen).
 */
export default function Breadcrumbs() {
  const crumbs = useBreadcrumbs();
  if (crumbs.length === 0) return null;

  return (
    <Breadcrumb.Root size="sm" minW={0} overflow="hidden">
      <Breadcrumb.List flexWrap="nowrap" whiteSpace="nowrap">
        {crumbs.map((c, i) => {
          const isLast = i === crumbs.length - 1;
          return (
            <Fragment key={`${c.to ?? "x"}-${i}`}>
              <Breadcrumb.Item
                minW={0}
                display={isLast ? "inline-flex" : { base: "none", md: "inline-flex" }}
              >
                {isLast ? (
                  <Breadcrumb.CurrentLink truncate maxW={{ base: "180px", md: "320px" }}>
                    {c.label}
                  </Breadcrumb.CurrentLink>
                ) : c.to ? (
                  <Breadcrumb.Link asChild color="fg.muted" _hover={{ color: "fg" }}>
                    <RouterLink to={c.to}>{c.label}</RouterLink>
                  </Breadcrumb.Link>
                ) : (
                  // Virtual crumb (a sidebar group like "Inventaris") — it names
                  // a section that has no page of its own, so it isn't a link.
                  <Text color="fg.muted">{c.label}</Text>
                )}
              </Breadcrumb.Item>
              {!isLast && (
                <Breadcrumb.Separator display={{ base: "none", md: "inline-flex" }}>
                  <ChevronRight size={14} />
                </Breadcrumb.Separator>
              )}
            </Fragment>
          );
        })}
      </Breadcrumb.List>
    </Breadcrumb.Root>
  );
}
