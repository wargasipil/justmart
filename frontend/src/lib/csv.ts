// Lightweight client-side CSV exporter for analytics tables.
// Serializes whatever rows the page currently has and triggers a download.

function escape(value: unknown): string {
  if (value === null || value === undefined) return "";
  const s = String(value);
  if (/[",\r\n]/.test(s)) {
    return `"${s.replace(/"/g, '""')}"`;
  }
  return s;
}

export function downloadCsv<T extends Record<string, unknown>>(
  filename: string,
  rows: T[],
  columns: { key: keyof T; header: string }[],
) {
  const header = columns.map((c) => escape(c.header)).join(",");
  const body = rows
    .map((row) => columns.map((c) => escape(row[c.key])).join(","))
    .join("\n");
  const csv = `${header}\n${body}`;

  const blob = new Blob(["﻿" + csv], { type: "text/csv;charset=utf-8" }); // BOM for Excel
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = filename;
  document.body.appendChild(a);
  a.click();
  document.body.removeChild(a);
  URL.revokeObjectURL(url);
}

// parseCsv parses CSV text (RFC-4180-ish) into header names + row objects keyed
// by the (trimmed, lowercased) header. Handles quoted fields with embedded
// commas / newlines / "" escapes, and \r\n | \n line endings; strips a BOM.
// Used by the product CSV import.
export function parseCsv(text: string): {
  headers: string[];
  rows: Record<string, string>[];
} {
  const records = parseRecords(text);
  if (records.length === 0) return { headers: [], rows: [] };
  const headers = records[0].map((h) => h.trim().toLowerCase());
  const rows: Record<string, string>[] = [];
  for (let i = 1; i < records.length; i++) {
    const cells = records[i];
    // Skip fully-blank lines (e.g. a trailing newline).
    if (cells.length === 1 && cells[0].trim() === "") continue;
    const row: Record<string, string> = {};
    headers.forEach((h, idx) => {
      row[h] = (cells[idx] ?? "").trim();
    });
    rows.push(row);
  }
  return { headers, rows };
}

// parseRecords splits CSV text into rows of raw fields, honoring quotes.
function parseRecords(text: string): string[][] {
  const src = text.charCodeAt(0) === 0xfeff ? text.slice(1) : text; // strip BOM
  const records: string[][] = [];
  let field = "";
  let row: string[] = [];
  let inQuotes = false;
  for (let i = 0; i < src.length; i++) {
    const ch = src[i];
    if (inQuotes) {
      if (ch === '"') {
        if (src[i + 1] === '"') {
          field += '"';
          i++; // skip the escaped quote
        } else {
          inQuotes = false;
        }
      } else {
        field += ch;
      }
      continue;
    }
    if (ch === '"') {
      inQuotes = true;
    } else if (ch === ",") {
      row.push(field);
      field = "";
    } else if (ch === "\n" || ch === "\r") {
      if (ch === "\r" && src[i + 1] === "\n") i++; // CRLF → one break
      row.push(field);
      records.push(row);
      field = "";
      row = [];
    } else {
      field += ch;
    }
  }
  // Flush the last field/row if the file didn't end with a newline.
  if (field !== "" || row.length > 0) {
    row.push(field);
    records.push(row);
  }
  return records;
}
