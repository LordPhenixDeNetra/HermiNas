interface ResultsTableProps {
  rows: Record<string, unknown>[];
}

export default function ResultsTable({ rows }: ResultsTableProps) {
  if (rows.length === 0) {
    return <p className="italic text-muted-fg">No rows</p>;
  }

  const columns = Object.keys(rows[0]);
  const cellClass = "whitespace-nowrap border border-border px-2.5 py-1.5 text-left";

  return (
    <table className="mt-2 block w-full overflow-x-auto border-collapse text-[13px]">
      <thead>
        <tr>
          {columns.map((c) => (
            <th key={c} className={`${cellClass} sticky top-0 bg-surface`}>
              {c}
            </th>
          ))}
        </tr>
      </thead>
      <tbody>
        {rows.map((row, i) => (
          <tr key={i}>
            {columns.map((c) => (
              <td key={c} className={cellClass}>
                {formatCell(row[c])}
              </td>
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
