import { useCallback, useEffect, useState } from "react";
import Editor from "@monaco-editor/react";
import { useQuery } from "@tanstack/react-query";
import { runQuery, listDatasets, ApiError, type QueryResult } from "../api/client";
import { useAuthStore } from "../store/auth";
import ResultsTable from "../components/ResultsTable";
import { registerSqlCompletionOnce, setCompletionDatasets } from "./sqlCompletion";
import { loadHistory, pushHistory } from "./queryHistory";
import { rowsToCsv, downloadFile } from "./exportResults";

export default function QueryStudio() {
  const token = useAuthStore((s) => s.token);
  const [sql, setSql] = useState("SELECT 1");
  const [result, setResult] = useState<QueryResult | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [running, setRunning] = useState(false);
  const [history, setHistory] = useState<string[]>(() => loadHistory());

  const { data: datasets } = useQuery({
    queryKey: ["datasets", token],
    queryFn: () => listDatasets(token as string),
    enabled: token !== null,
  });

  useEffect(() => {
    setCompletionDatasets(datasets ?? []);
  }, [datasets]);

  const handleRun = useCallback(async () => {
    if (!token || !sql.trim()) return;
    setRunning(true);
    setError(null);
    try {
      const res = await runQuery(sql, token);
      setResult(res);
      setHistory((prev) => pushHistory(prev, sql));
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Query failed");
      setResult(null);
    } finally {
      setRunning(false);
    }
  }, [sql, token]);

  const exportCsv = useCallback(() => {
    if (!result || result.rows.length === 0) return;
    downloadFile("results.csv", rowsToCsv(result.rows), "text/csv");
  }, [result]);

  const exportJson = useCallback(() => {
    if (!result) return;
    downloadFile("results.json", JSON.stringify(result.rows, null, 2), "application/json");
  }, [result]);

  return (
    <div className="grid items-start gap-5 grid-cols-[1fr_240px] max-[900px]:grid-cols-1">
      <div className="col-start-1 overflow-hidden rounded-lg border border-border">
        <Editor
          height="240px"
          defaultLanguage="sql"
          value={sql}
          onChange={(value) => setSql(value ?? "")}
          onMount={(_editor, monaco) => registerSqlCompletionOnce(monaco)}
          options={{ minimap: { enabled: false }, fontSize: 14, automaticLayout: true }}
        />
        <div className="flex gap-2 border-t border-border bg-surface p-2.5">
          <button onClick={handleRun} className="btn btn-primary" disabled={running || !sql.trim()}>
            {running ? "Running…" : "Run query"}
          </button>
          <button onClick={exportCsv} className="btn" disabled={!result || result.rows.length === 0}>
            Export CSV
          </button>
          <button onClick={exportJson} className="btn" disabled={!result}>
            Export JSON
          </button>
        </div>
      </div>

      {error && (
        <p role="alert" className="rounded-md bg-danger-bg px-3 py-2 text-danger">
          {error}
        </p>
      )}

      {result && (
        <div className="col-start-1">
          <p>
            {result.row_count} row{result.row_count === 1 ? "" : "s"}
            {result.cached ? " (cached)" : ""}
          </p>
          <ResultsTable rows={result.rows} />
        </div>
      )}

      <aside className="col-start-2 flex flex-col gap-5 max-[900px]:col-start-1">
        <section>
          <h2 className="mb-2 text-[13px] uppercase tracking-[0.06em] text-muted-fg">History</h2>
          {history.length === 0 ? (
            <p className="italic text-muted-fg">No queries yet</p>
          ) : (
            <ul className="flex flex-col gap-1">
              {history.map((q) => (
                <li key={q}>
                  <button onClick={() => setSql(q)} title={q} className="w-full cursor-pointer truncate bg-transparent text-left">
                    {q}
                  </button>
                </li>
              ))}
            </ul>
          )}
        </section>

        <section>
          <h2 className="mb-2 text-[13px] uppercase tracking-[0.06em] text-muted-fg">Datasets</h2>
          {!datasets || datasets.length === 0 ? (
            <p className="italic text-muted-fg">No datasets yet</p>
          ) : (
            <ul className="flex flex-col gap-1">
              {datasets.map((d) => (
                <li key={d.name}>{d.name}</li>
              ))}
            </ul>
          )}
        </section>
      </aside>
    </div>
  );
}
