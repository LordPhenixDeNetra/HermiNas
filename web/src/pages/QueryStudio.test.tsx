import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import QueryStudio from "./QueryStudio";
import { useAuthStore } from "../store/auth";

// Monaco doesn't render meaningfully in jsdom (no real editor, no canvas
// layout) — swapped for a plain textarea so the surrounding logic (run,
// export, history, results) is exercised for real, which is what these
// tests actually care about. registerSqlCompletionOnce still gets called,
// same as with the real editor's onMount.
vi.mock("@monaco-editor/react", () => ({
  default: ({
    value,
    onChange,
    onMount,
  }: {
    value: string;
    onChange: (v: string) => void;
    onMount?: (editor: unknown, monaco: unknown) => void;
  }) => {
    onMount?.(
      {},
      {
        languages: {
          registerCompletionItemProvider: vi.fn(),
          CompletionItemKind: { Class: 1, Field: 2 },
        },
      },
    );
    return <textarea aria-label="sql-editor" value={value} onChange={(e) => onChange(e.target.value)} />;
  },
}));

function renderQueryStudio() {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={queryClient}>
      <QueryStudio />
    </QueryClientProvider>,
  );
}

describe("QueryStudio", () => {
  beforeEach(() => {
    useAuthStore.getState().setSession("token-123", "alice", "engineer");
    localStorage.removeItem("herminas-query-history");
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("runs a query and renders the results table", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockImplementation((path: string) => {
        if (path === "/api/v1/datasets") {
          return Promise.resolve({ ok: true, text: async () => JSON.stringify([]) });
        }
        return Promise.resolve({
          ok: true,
          text: async () =>
            JSON.stringify({ row_count: 1, cached: false, rows: [{ number: "1" }] }),
        });
      }),
    );

    renderQueryStudio();
    await userEvent.click(screen.getByRole("button", { name: "Run query" }));

    await waitFor(() => expect(screen.getByText("1 row")).toBeInTheDocument());
    expect(screen.getByRole("columnheader", { name: "number" })).toBeInTheDocument();
  });

  it("shows an error message when the query fails", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockImplementation((path: string) => {
        if (path === "/api/v1/datasets") {
          return Promise.resolve({ ok: true, text: async () => JSON.stringify([]) });
        }
        return Promise.resolve({
          ok: false,
          status: 400,
          statusText: "Bad Request",
          text: async () => JSON.stringify({ error: "syntax error near FROM" }),
        });
      }),
    );

    renderQueryStudio();
    await userEvent.click(screen.getByRole("button", { name: "Run query" }));

    await waitFor(() => expect(screen.getByRole("alert")).toHaveTextContent("syntax error near FROM"));
  });

  it("adds a successful query to history and re-running from history repopulates the editor", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockImplementation((path: string) => {
        if (path === "/api/v1/datasets") {
          return Promise.resolve({ ok: true, text: async () => JSON.stringify([]) });
        }
        return Promise.resolve({
          ok: true,
          text: async () => JSON.stringify({ row_count: 0, cached: false, rows: [] }),
        });
      }),
    );

    renderQueryStudio();
    const editor = screen.getByLabelText("sql-editor");
    await userEvent.clear(editor);
    await userEvent.type(editor, "SELECT 42");
    await userEvent.click(screen.getByRole("button", { name: "Run query" }));

    await waitFor(() => expect(screen.getByRole("button", { name: "SELECT 42" })).toBeInTheDocument());
  });

  it("disables export buttons until a result exists", () => {
    renderQueryStudio();
    expect(screen.getByRole("button", { name: "Export CSV" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "Export JSON" })).toBeDisabled();
  });

  it("lists datasets fetched from the API", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue({
        ok: true,
        text: async () =>
          JSON.stringify([{ name: "app_logs", columns: [{ name: "message", type: "String", nullable: false }] }]),
      }),
    );

    renderQueryStudio();
    await waitFor(() => expect(screen.getByText("app_logs")).toBeInTheDocument());
  });
});
