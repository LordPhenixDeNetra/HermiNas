interface ResultsTableProps {
  rows: Record<string, unknown>[];
}

export default function ResultsTable({ rows }: ResultsTableProps) {
  if (rows.length === 0) {
    return <p className="empty-state">No rows</p>;
  }

  const columns = Object.keys(rows[0]);

  return (
    <table className="results-table">
      <thead>
        <tr>
          {columns.map((c) => (
            <th key={c}>{c}</th>
          ))}
        </tr>
      </thead>
      <tbody>
        {rows.map((row, i) => (
          <tr key={i}>
            {columns.map((c) => (
              <td key={c}>{formatCell(row[c])}</td>
            ))}
          </tr>
        ))}
      </tbody>
    </table>
  );
}

function formatCell(value: unknown): string {
  if (value === null || value === undefined) return "";
  if (typeof value === "object") return JSON.stringify(value);
  return String(value);
}
