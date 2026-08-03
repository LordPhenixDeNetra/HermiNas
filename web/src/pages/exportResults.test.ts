import { describe, expect, it } from "vitest";
import { rowsToCsv } from "./exportResults";

describe("rowsToCsv", () => {
  it("returns an empty string for no rows", () => {
    expect(rowsToCsv([])).toBe("");
  });

  it("writes a header row followed by data rows", () => {
    const csv = rowsToCsv([
      { id: 1, name: "alice" },
      { id: 2, name: "bob" },
    ]);
    expect(csv).toBe("id,name\n1,alice\n2,bob");
  });

  it("quotes values containing commas or quotes", () => {
    const csv = rowsToCsv([{ message: 'hello, "world"' }]);
    expect(csv).toBe('message\n"hello, ""world"""');
  });

  it("renders null/undefined as an empty cell", () => {
    const csv = rowsToCsv([{ a: null, b: undefined }]);
    expect(csv).toBe("a,b\n,");
  });

  it("stringifies nested objects as JSON", () => {
    const csv = rowsToCsv([{ meta: { nested: "value" } }]);
    expect(csv).toContain('"{""nested"":""value""}"');
  });
});
