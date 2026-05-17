import { readdirSync, statSync } from "fs";
import { join } from "path";

const DIST_DIR = process.argv[2] || "dist/assets";
const BUDGETS = {
  "monaco-editor": 3 * 1024 * 1024,
  "sql-tools":     500 * 1024,
  "ui-framework":  800 * 1024,
  "utils":         300 * 1024,
  "react-core":    500 * 1024,
  "main":          1.5 * 1024 * 1024,
};

let failed = false;
try {
  const files = readdirSync(DIST_DIR);

  for (const [chunk, maxBytes] of Object.entries(BUDGETS)) {
    const match = files.find(f => f.includes(chunk) && f.endsWith(".js"));
    if (!match) continue;
    const size = statSync(join(DIST_DIR, match)).size;
    const status = size > maxBytes ? "FAIL ❌" : "OK ✓";
    console.log(`${chunk}: ${(size/1024).toFixed(0)}KB / ${(maxBytes/1024).toFixed(0)}KB ${status}`);
    if (size > maxBytes) failed = true;
  }
} catch (e) {
  console.error(`Could not read directory ${DIST_DIR}`, e.message);
  process.exit(1);
}

if (failed) { console.error("\nBundle budget exceeded!"); process.exit(1); }
console.log("\nAll chunks within budget ✓");
