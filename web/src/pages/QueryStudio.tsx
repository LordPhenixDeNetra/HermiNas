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
    <div className="query-studio">
      <div className="query-studio-editor">
        <Editor
          height="240px"
          defaultLanguage="sql"
          value={sql}
          onChange={(value) => setSql(value ?? "")}
          onMount={(_editor, monaco) => registerSqlCompletionOnce(monaco)}
          options={{ minimap: { enabled: false }, fontSize: 14, automaticLayout: true }}
        />
        <div className="query-studio-actions">
          <button onClick={handleRun} disabled={running || !sql.trim()}>
            {running ? "Running…" : "Run query"}
          </button>
          <button onClick={exportCsv} disabled={!result || result.rows.length === 0}>
            Export CSV
          </button>
          <button onClick={exportJson} disabled={!result}>
            Export JSON
          </button>
        </div>
      </div>

      {error && (
        <p role="alert" className="error">
          {error}
        </p>
      )}

      {result && (
        <div className="query-studio-results">
          <p>
            {result.row_count} row{result.row_count === 1 ? "" : "s"}
            {result.cached ? " (cached)" : ""}
          </p>
          <ResultsTable rows={result.rows} />
        </div>
      )}

      <aside className="query-studio-sidebar">
        <section>
          <h2>History</h2>
          {history.length === 0 ? (
            <p className="empty-state">No queries yet</p>
          ) : (
            <ul>
              {history.map((q, i) => (
                <li key={i}>
                  <button onClick={() => setSql(q)}>{q}</button>
                </li>
              ))}
            </ul>
          )}
        </section>

        <section>
          <h2>Datasets</h2>
          {!datasets || datasets.length === 0 ? (
            <p className="empty-state">No datasets yet</p>
          ) : (
            <ul>
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
