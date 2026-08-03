// This monaco-editor version ships the SQL tokenizer as a plain JS module
// with no accompanying .d.ts (unlike its sibling register.js) — see the
// comment in monacoSetup.ts for why this is imported directly rather than
// via a self-registering "contribution" module.
declare module "monaco-editor/languages/definitions/sql/sql" {
  import type { languages } from "monaco-editor/editor/editor.api";

  export const conf: languages.LanguageConfiguration;
  export const language: languages.IMonarchLanguage;
}
