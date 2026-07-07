/**
 * Loads the shared, language-neutral compliance fixtures
 * (testkit/compliance-tests/fixtures) that define TenantGuard conformance across all
 * language bindings. TypeScript mirror of the Go/Java/Python loaders.
 */
import { existsSync, readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

export function fixturesDir(): string {
  let dir = dirname(fileURLToPath(import.meta.url));
  for (;;) {
    const candidate = join(dir, "testkit", "compliance-tests", "fixtures");
    if (existsSync(candidate)) {
      return candidate;
    }
    const parent = dirname(dir);
    if (parent === dir) {
      throw new Error("could not locate testkit/compliance-tests/fixtures");
    }
    dir = parent;
  }
}

export function load(name: string): any {
  return JSON.parse(readFileSync(join(fixturesDir(), name), "utf8"));
}

export function cases(name: string): any[] {
  return load(name).cases;
}
