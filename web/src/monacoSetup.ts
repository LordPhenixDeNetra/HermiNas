// Self-hosts Monaco (M1.6's SQL editor) instead of letting
// @monaco-editor/react's default loader fetch it from
// cdn.jsdelivr.net at runtime — found by inspecting the production
// build, where that CDN URL showed up baked into the bundle. HermiNas is
// self-hosted and explicitly supports air-gapped deployment (cahier des
// charges §7.3); a UI component silently phoning out to a public CDN on
// every load would break that promise outright, not just bend it. Import
// this once, before the app renders (see main.tsx).
//
// Importing the full `monaco-editor` barrel pulls in every bundled
// language (60+ chunks, ~4MB) — Query Studio only ever edits SQL, so this
// imports just the editor core plus the SQL tokenizer definition. This
// monaco-editor version doesn't ship a self-registering "sql.contribution"
// side-effect module (that's how older versions/tutorials do it) — its
// basic-languages moved under languages/definitions and export plain
// {conf, language} objects that must be registered by hand.
import * as monaco from "monaco-editor/editor/editor.api";
import { conf as sqlConf, language as sqlLanguage } from "monaco-editor/languages/definitions/sql/sql";
import { loader } from "@monaco-editor/react";
// eslint-disable-next-line import/no-unresolved -- Vite's `?worker` suffix
import EditorWorker from "monaco-editor/editor/editor.worker?worker";

monaco.languages.register({ id: "sql" });
monaco.languages.setLanguageConfiguration("sql", sqlConf);
monaco.languages.setMonarchTokensProvider("sql", sqlLanguage);

self.MonacoEnvironment = {
  getWorker() {
    return new EditorWorker();
  },
};

loader.config({ monaco });
