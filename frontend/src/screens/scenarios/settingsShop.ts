import { Code } from "@connectrpc/connect";
import type { HttpHandler } from "msw";

import { BackupService } from "../../gen/backup_iface/v1/backup_connect";
import { ConnectorService } from "../../gen/connector_iface/v1/connector_connect";
import { SettingsService } from "../../gen/settings_iface/v1/settings_connect";
import { BussinessType } from "../../gen/settings_iface/v1/settings_pb";
import { UnitService } from "../../gen/unit_iface/v1/unit_connect";
import { UnitBase, UnitDerivative } from "../../gen/unit_iface/v1/unit_pb";
import { RETAIL_CATALOG, daysAgo, unitBasesFor } from "../../routes/dev/fixtures";
import { mockRpc, rpcError } from "../../routes/dev/storyMocks";

// The fake shop behind the /settings stories — one writeable store plus the
// handlers for all six panels. The states and their prose live next door in
// scenarios/settings.tsx; this file is only "the server".
//
// WHY THE MOCKS WRITE. Five of the six panels are forms, and a canned read
// makes Save look broken: the mutation invalidates its key, the refetch serves
// the fixture again, and the field visibly snaps back. Writing to a per-story
// store instead means the page behaves — renaming the shop re-brands the
// sidebar and the browser tab, switching the business mode grows the Resep nav
// item and turns Produk into Obat, Create/Delete backup moves a row. Same
// reasoning as the Products story mocking ListProducts as a function of the
// request rather than as a fixed page.
//
// The rules the server enforces are mirrored here (title length, tunnel-token
// shape) rather than waved through, because the panels' error states are the
// interesting half and a mock that accepts everything makes them dead code.

const DEFAULT_LIMIT = 25;

// --- state ------------------------------------------------------------------

export type Tunnel = {
  configured: boolean;
  tokenPreview: string;
  active: boolean;
  /** "none" | "settings" | "config" | "env" | "env_off" */
  source: string;
  restartRequired: boolean;
  supported: boolean;
};

/** No token anywhere — a shop that has never opened this panel. */
export const TUNNEL_OFF: Tunnel = {
  configured: false,
  tokenPreview: "",
  active: false,
  source: "none",
  restartRequired: false,
  supported: true,
};

/** A saved token, masked the way the server masks it. */
export const TUNNEL_SAVED = { configured: true, tokenPreview: "eyJhIj••••In0=" };

export const CONNECTORS = [
  { deviceId: "kasir-1", deviceName: "PC Kasir 1", printerNames: ["EPSON TM-T82 Receipt", "Label Rak"] },
  { deviceId: "gudang", deviceName: "PC Gudang", printerNames: ["POS-58"] },
];

/** `count` snapshots, newest first and one a day — the server's own order. */
function backupsFixture(count: number) {
  return Array.from({ length: count }, (_, i) => {
    const at = daysAgo(i);
    const d = new Date(Number(at) * 1000);
    const p2 = (n: number) => String(n).padStart(2, "0");
    const stamp =
      `${d.getFullYear()}-${p2(d.getMonth() + 1)}-${p2(d.getDate())}` +
      `_${p2(d.getHours())}${p2(d.getMinutes())}${p2(d.getSeconds())}`;
    return {
      name: `backup_${stamp}`,
      createdAt: at,
      sizeBytes: BigInt(4_200_000 - i * 37_000),
      schemaVersion: 58,
    };
  });
}

function newShop() {
  return {
    settings: { lowStockThreshold: 10, appTitle: "Toko Jaya", businessType: BussinessType.RETAIL },
    printTarget: { connectorDeviceId: "kasir-1", printerName: "EPSON TM-T82 Receipt" },
    receipt: {
      header: "TOKO JAYA\nJl. Merdeka 1, Bandung\n0812-3456-7890",
      footer: "Terima kasih!\nBarang yang sudah dibeli tidak dapat ditukar.",
      width: 32,
    },
    tunnel: { ...TUNNEL_OFF },
    backups: backupsFixture(7),
    bases: unitBasesFor(RETAIL_CATALOG),
  };
}
type Shop = ReturnType<typeof newShop>;

// --- mirrored server rules --------------------------------------------------

const MAX_APP_TITLE_LEN = 60; // settings.MaxAppTitleLen

/** common.NormalizeTunnelToken: base64url-ish, 32–4096 chars, nothing stripped out. */
function normalizeToken(raw: string): string | null {
  const s = raw.trim();
  if (s === "") return ""; // clearing the setting is always valid
  if (s.length < 32 || s.length > 4096) return null;
  return /^[A-Za-z0-9+/=\-_.]+$/.test(s) ? s : null;
}

/** common.MaskTunnelToken: enough to recognise which token is saved, never enough to use. */
function maskToken(token: string): string {
  if (token === "") return "";
  if (token.length <= 12) return "•".repeat(token.length);
  return `${token.slice(0, 6)}••••${token.slice(-4)}`;
}

// --- the version fixtures the Updates panel is staged with ------------------

export const UP_TO_DATE = {
  currentVersion: "v1.9.0",
  checked: true,
  updateAvailable: false,
  latestVersion: "v1.9.0",
  enabled: true,
  canSelfApply: true,
};

export const UPDATE_AVAILABLE = {
  currentVersion: "v1.8.2",
  checked: true,
  updateAvailable: true,
  latestVersion: "v1.9.0",
  publishedAt: daysAgo(2),
  releaseUrl: "https://github.com/justmart/justmart/releases/tag/v1.9.0",
  releaseNotes:
    "Grosir: harga bertingkat per satuan, lengkap dengan riwayat harga.\n" +
    "Restock: penerimaan yang salah input bisa dibatalkan selama lot belum tersentuh.\n" +
    "Struk: header dan footer bisa diatur dari Pengaturan ▸ Pencetakan.\n" +
    "Perbaikan: laporan harian tidak lagi menghitung penjualan dini hari ke tanggal sebelumnya.",
  enabled: true,
  canSelfApply: true,
  canRevert: true,
  backupVersion: "v1.8.1",
};

// --- handlers ---------------------------------------------------------------

function generalHandlers(shop: Shop): HttpHandler[] {
  // GetBranding and GetBussinessSettings are served from the SAME store as
  // GetSettings: UpdateSettings invalidates all three, so this is what makes a
  // saved title or mode re-theme the shell live instead of the shell snapping
  // back to a fixture brand.
  return [
    mockRpc(SettingsService, "getSettings", () => ({ settings: shop.settings })),
    mockRpc(SettingsService, "getBranding", () => ({
      businessType: shop.settings.businessType,
      appTitle: shop.settings.appTitle,
    })),
    mockRpc(SettingsService, "getBussinessSettings", () => ({
      type: shop.settings.businessType,
      appTitle: shop.settings.appTitle,
    })),
    mockRpc(SettingsService, "updateSettings", (req) => {
      const appTitle = req.appTitle.trim();
      if ([...appTitle].length > MAX_APP_TITLE_LEN) {
        return rpcError(Code.InvalidArgument, "settings.app_title_too_long");
      }
      shop.settings = {
        lowStockThreshold: req.lowStockThreshold,
        appTitle,
        // UNSPECIFIED means "leave the mode alone", so a threshold-only save
        // cannot silently reset a pharmacy shop to retail.
        businessType:
          req.businessType === BussinessType.UNSPECIFIED ? shop.settings.businessType : req.businessType,
      };
      return { settings: shop.settings };
    }),
  ];
}

function unitHandlers(shop: Shop): HttpHandler[] {
  const withBases = (b: UnitBase) =>
    new UnitBase({ id: b.id, name: b.name, active: b.active, createdAt: b.createdAt, derivatives: b.derivatives });
  return [
    mockRpc(UnitService, "listUnitBases", (req) => {
      const limit = req.limit > 0 ? req.limit : DEFAULT_LIMIT;
      return { bases: shop.bases.slice(req.offset, req.offset + limit), total: shop.bases.length };
    }),
    mockRpc(UnitService, "createUnitBase", (req) => {
      const base = new UnitBase({
        id: `ub-new-${shop.bases.length + 1}`,
        name: req.name,
        active: true,
        createdAt: daysAgo(0),
      });
      shop.bases = [...shop.bases, base];
      return { base };
    }),
    mockRpc(UnitService, "archiveUnitBase", (req) => {
      shop.bases = shop.bases.filter((b) => b.id !== req.id);
      return {};
    }),
    mockRpc(UnitService, "createUnitDerivative", (req) => {
      const derivative = new UnitDerivative({
        id: `ud-new-${shop.bases.length}-${req.name}`,
        baseUnitId: req.baseUnitId,
        name: req.name,
        factor: req.factor,
        sortOrder: req.sortOrder,
        active: true,
      });
      shop.bases = shop.bases.map((b) => {
        if (b.id !== req.baseUnitId) return b;
        const next = withBases(b);
        next.derivatives = [...b.derivatives, derivative];
        return next;
      });
      return { derivative };
    }),
    mockRpc(UnitService, "archiveUnitDerivative", (req) => {
      shop.bases = shop.bases.map((b) => {
        const next = withBases(b);
        next.derivatives = b.derivatives.filter((d) => d.id !== req.id);
        return next;
      });
      return {};
    }),
  ];
}

function printingHandlers(shop: Shop): HttpHandler[] {
  return [
    // Connector mode is the default staging; a story overrides this one read to
    // show the usb and tcp branches instead.
    mockRpc(SettingsService, "getPrintingInfo", { mode: "connector", localPrinters: [] }),
    mockRpc(ConnectorService, "listConnectors", { connectors: CONNECTORS }),
    mockRpc(SettingsService, "getPrintTarget", () => shop.printTarget),
    mockRpc(SettingsService, "setPrintTarget", (req) => {
      shop.printTarget = { connectorDeviceId: req.connectorDeviceId, printerName: req.printerName };
      return shop.printTarget;
    }),
    mockRpc(SettingsService, "getReceiptSettings", () => shop.receipt),
    mockRpc(SettingsService, "setReceiptSettings", (req) => {
      shop.receipt = { header: req.header, footer: req.footer, width: req.width };
      return shop.receipt;
    }),
  ];
}

function tunnelHandlers(shop: Shop): HttpHandler[] {
  // The token shape is checked exactly as the server checks it, so pasting the
  // `cloudflared service install …` line Cloudflare shows — the likeliest
  // wrong input, and the one the FAQ video films — lands the real field error
  // rather than storing a credential nobody validated.
  return [
    mockRpc(SettingsService, "getTunnelSettings", () => shop.tunnel),
    mockRpc(SettingsService, "setTunnelSettings", (req) => {
      const token = normalizeToken(req.token);
      if (token === null) return rpcError(Code.InvalidArgument, "settings.tunnel_token_invalid");
      shop.tunnel = token
        ? {
            ...shop.tunnel,
            configured: true,
            tokenPreview: maskToken(token),
            // Saving never starts a tunnel — the token is read once, at boot…
            restartRequired: true,
            // …and it never outranks the environment's off-switch.
            source: shop.tunnel.source === "env_off" ? "env_off" : "settings",
          }
        : {
            ...shop.tunnel,
            configured: false,
            tokenPreview: "",
            // A running tunnel keeps running until the process restarts.
            restartRequired: shop.tunnel.active,
          };
      return shop.tunnel;
    }),
  ];
}

function backupHandlers(shop: Shop): HttpHandler[] {
  return [
    mockRpc(BackupService, "listBackups", (req) => {
      const limit = req.limit > 0 ? req.limit : DEFAULT_LIMIT;
      return { backups: shop.backups.slice(req.offset, req.offset + limit), total: shop.backups.length };
    }),
    mockRpc(BackupService, "createBackup", () => {
      const [backup] = backupsFixture(1);
      shop.backups = [backup, ...shop.backups];
      return { backup };
    }),
    mockRpc(BackupService, "deleteBackup", (req) => {
      shop.backups = shop.backups.filter((b) => b.name !== req.name);
      return {};
    }),
  ];
}

function updateHandlers(): HttpHandler[] {
  // CheckUpdate is also read by the shell's bell (owner only), so an
  // update-available story badges the TopBar too — exactly as the app does.
  return [
    mockRpc(SettingsService, "checkUpdate", UP_TO_DATE),
    mockRpc(SettingsService, "applyUpdate", { stagedVersion: "v1.9.0", restarting: false }),
    mockRpc(SettingsService, "revertUpdate", { staged: true }),
  ];
}

/** Everything the six panels read and write, over one fresh fake shop. */
export function shopHandlers(): HttpHandler[] {
  const shop = newShop();
  return [
    ...generalHandlers(shop),
    ...unitHandlers(shop),
    ...printingHandlers(shop),
    ...tunnelHandlers(shop),
    ...backupHandlers(shop),
    ...updateHandlers(),
  ];
}
