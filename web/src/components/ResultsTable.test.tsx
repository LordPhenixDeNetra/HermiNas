import { describe, expect, it } from "vitest";
import { render, screen } from "@testing-library/react";
import ResultsTable from "./ResultsTable";

describe("ResultsTable", () => {
  it("shows an empty state for no rows", () => {
    render(<ResultsTable rows={[]} />);
    expect(screen.getByText("No rows")).toBeInTheDocument();
  });

  it("renders a header per column and one row per record", () => {
    render(
      <ResultsTable
        rows={[
          { id: "1", message: "hello" },
          { id: "2", message: "world" },
        ]}
      />,
    );
    expect(screen.getByRole("columnheader", { name: "id" })).toBeInTheDocument();
    expect(screen.getByRole("columnheader", { name: "message" })).toBeInTheDocument();
    expect(screen.getByText("hello")).toBeInTheDocument();
    expect(screen.getByText("world")).toBeInTheDocument();
  });

  it("renders null/undefined cells as blank rather than the literal string", () => {
    render(<ResultsTable rows={[{ a: null, b: undefined }]} />);
    const cells = screen.getAllByRole("cell");
    expect(cells.map((c) => c.textContent)).toEqual(["", ""]);
  });

  it("stringifies nested object cells", () => {
    render(<ResultsTable rows={[{ meta: { nested: "value" } }]} />);
    expect(screen.getByText('{"nested":"value"}')).toBeInTheDocument();
  });
});
