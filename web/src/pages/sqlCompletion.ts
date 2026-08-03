import type { Dataset } from "../api/client";

// Monaco's completion provider is registered once per language for the
// whole editor instance, not per mounted <Editor>; re-registering on every
// mount (e.g. React StrictMode's double-invoke, or navigating away and
// back) would stack duplicate providers and show every suggestion twice.
// `currentDatasets` is a module-level box the already-registered provider
// reads from, updated on every dataset fetch via `setCompletionDatasets`.
let registered = false;
let currentDatasets: Dataset[] = [];

export function setCompletionDatasets(datasets: Dataset[]) {
  currentDatasets = datasets;
}

// `monaco` here is the module from `@monaco-editor/react`'s onMount second
// argument — typed loosely (not importing monaco-editor's own types as a
// direct dependency) to keep this file decoupled from the editor package's
// exact type surface across versions.
export function registerSqlCompletionOnce(monaco: {
  languages: {
    registerCompletionItemProvider: (languageId: string, provider: unknown) => void;
    CompletionItemKind: { Class: number; Field: number };
  };
}) {
  if (registered) return;
  registered = true;

  monaco.languages.registerCompletionItemProvider("sql", {
    provideCompletionItems(
      model: { getWordUntilPosition: (pos: unknown) => { startColumn: number; endColumn: number } },
      position: { lineNumber: number },
    ) {
      const word = model.getWordUntilPosition(position);
      const range = {
        startLineNumber: position.lineNumber,
        endLineNumber: position.lineNumber,
        startColumn: word.startColumn,
        endColumn: word.endColumn,
      };

      const suggestions: unknown[] = [];
      for (const dataset of currentDatasets) {
        suggestions.push({
          label: dataset.name,
          kind: monaco.languages.CompletionItemKind.Class,
          insertText: dataset.name,
          detail: "dataset",
          range,
        });
        for (const column of dataset.columns) {
          suggestions.push({
            label: column.name,
            kind: monaco.languages.CompletionItemKind.Field,
            insertText: column.name,
            detail: `${dataset.name}.${column.name}: ${column.type}`,
            range,
          });
        }
      }
      return { suggestions };
    },
  });
}
