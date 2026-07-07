/** Unit tests for schema isolation. */
import { describe, expect, it } from "vitest";
import { TenantContext } from "../src/context/index.js";
import { IsolationError, SchemaStrategy, UNSAFE_TENANT_ID } from "../src/isolation/index.js";

describe("SchemaStrategy", () => {
  it("quotes the schema in the SET statement", () => {
    const sql = new SchemaStrategy("tenant_").searchPathSql(TenantContext.create("acme"));
    expect(sql).toBe('SET search_path TO "tenant_acme"');
  });

  it("rejects injection in the tenant id", () => {
    try {
      new SchemaStrategy("tenant_").schemaName(
        TenantContext.create('acme"; DROP SCHEMA public CASCADE; --'),
      );
      throw new Error("expected throw");
    } catch (e) {
      expect(e).toBeInstanceOf(IsolationError);
      expect((e as IsolationError).code).toBe(UNSAFE_TENANT_ID);
    }
  });

  it("binds by executing the SET statement", async () => {
    const executed: string[] = [];
    const runner = {
      query: (sql: string) => {
        executed.push(sql);
        return Promise.resolve();
      },
    };
    await new SchemaStrategy("tenant_").bind(runner, TenantContext.create("acme"));
    expect(executed).toEqual(['SET search_path TO "tenant_acme"']);
  });
});
