import { Box, Text } from "@chakra-ui/react";
import { useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";

import { renderBarcodeInto, type BarcodeOptions } from "../lib/barcode";

export type BarcodeProps = BarcodeOptions & {
  /** The data to encode — a product SKU here, since POS scans by exact SKU. */
  value: string;
};

/**
 * CODE128 barcode as inline SVG.
 *
 * Always renders black-on-white regardless of theme: a barcode is an optical
 * target, so inverting it in dark mode makes it unscannable. When `value` can't
 * be encoded, it renders a plain warning instead of a symbol that looks
 * scannable but isn't.
 */
export default function Barcode({ value, ...opts }: BarcodeProps) {
  const { t } = useTranslation();
  const ref = useRef<SVGSVGElement>(null);
  const [ok, setOk] = useState(true);

  useEffect(() => {
    if (!ref.current) return;
    setOk(renderBarcodeInto(ref.current, value, opts));
    // opts is a fresh object each render; spreading its values keeps the effect
    // from re-running on every parent render.
  }, [value, opts.width, opts.height, opts.displayValue, opts.fontSize, opts.margin]);

  if (!ok) {
    return (
      <Text fontSize="sm" color="fg.error">
        {t("barcode.notEncodable")}
      </Text>
    );
  }
  return (
    <Box bg="white" display="inline-block" lineHeight={0}>
      <svg ref={ref} />
    </Box>
  );
}
