#!/usr/bin/env node
/**
 * Semantic Token Checker — scans .tsx files for raw Tailwind color classes.
 * Usage: node scripts/check-semantic-tokens.mjs
 *
 * Reports violations like: bg-blue-500, text-red-600, border-gray-300
 * Suggests using semantic tokens like: bg-accent, text-error, border-control-border
 */

import { readFileSync, readdirSync, statSync } from "fs";
import { join, relative } from "path";

const RAW_COLOR_PATTERN =
  /\b(bg|text|border|ring|outline|shadow|accent|fill|stroke)-(blue|red|green|yellow|purple|gray|slate|zinc|stone|neutral|orange|pink|cyan|teal|indigo|violet|rose|amber|lime|emerald|sky|fuchsia)-\d{2,3}\b/g;

const SEMANTIC_SUGGESTIONS = {
  bg: "bg-accent, bg-main, bg-control, bg-control-bg",
  text: "text-main-text, text-control-placeholder, text-accent, text-error",
  border: "border-control-border, border-accent",
  ring: "ring-accent, ring-error",
};

const SRC_DIR = join(process.cwd(), "src", "react");

let totalViolations = 0;
const violations = [];

function scanFile(filePath) {
  const content = readFileSync(filePath, "utf-8");
  const lines = content.split("\n");

  lines.forEach((line, lineIndex) => {
    let match;
    RAW_COLOR_PATTERN.lastIndex = 0;
    while ((match = RAW_COLOR_PATTERN.exec(line)) !== null) {
      const className = match[0];
      const prefix = match[1];
      const relPath = relative(process.cwd(), filePath);
      violations.push({
        file: relPath,
        line: lineIndex + 1,
        className,
        suggestion: SEMANTIC_SUGGESTIONS[prefix] || "Check design system tokens",
      });
      totalViolations++;
    }
  });
}

function scanDirectory(dir) {
  try {
    const entries = readdirSync(dir);
    for (const entry of entries) {
      const fullPath = join(dir, entry);
      const stat = statSync(fullPath);
      if (stat.isDirectory()) {
        if (entry === "node_modules" || entry === "dist") continue;
        scanDirectory(fullPath);
      } else if (entry.endsWith(".tsx")) {
        scanFile(fullPath);
      }
    }
  } catch {
    // Skip inaccessible directories
  }
}

console.log("🔍 Scanning .tsx files for raw color classes...\n");
scanDirectory(SRC_DIR);

if (violations.length === 0) {
  console.log("✅ No raw color classes found. All files use semantic tokens.");
  process.exit(0);
} else {
  console.log(`⚠️  Found ${totalViolations} raw color class violation(s):\n`);
  for (const v of violations) {
    console.log(`  ${v.file}:${v.line}  →  ${v.className}`);
    console.log(`    💡 Use instead: ${v.suggestion}\n`);
  }
  console.log(`\nTotal: ${totalViolations} violation(s) in ${new Set(violations.map((v) => v.file)).size} file(s).`);
  // Exit with warning (non-blocking for CI initially)
  process.exit(0);
}
