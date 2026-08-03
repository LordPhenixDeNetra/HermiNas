import { useEffect } from "react";
import { useThemeStore } from "../store/theme";

// Stamps the chosen theme onto <html data-theme="..."> so every page —
// including Login, which sits outside the themed app shell — respects the
// user's explicit choice, not just pages wrapped by Layout.
export function useSyncTheme() {
  const theme = useThemeStore((s) => s.theme);
  useEffect(() => {
    document.documentElement.dataset.theme = theme;
  }, [theme]);
}
