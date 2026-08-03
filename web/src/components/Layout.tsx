import { NavLink, Outlet, useNavigate } from "react-router-dom";
import { useThemeStore } from "../store/theme";
import { useAuthStore } from "../store/auth";

// Inline SVG instead of a 🌙/☀️ emoji glyph: emoji rendering (weight, size,
// color) varies across OS/browser font stacks, so it can't reliably match
// the button's currentColor or the rest of the icon-free UI.
function ThemeIcon({ theme }: { theme: "light" | "dark" }) {
  if (theme === "light") {
    return (
      <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
        <path d="M21 12.79A9 9 0 1 1 11.21 3 7 7 0 0 0 21 12.79Z" />
      </svg>
    );
  }
  return (
    <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <circle cx="12" cy="12" r="4" />
      <path d="M12 2v2M12 20v2M4.93 4.93l1.41 1.41M17.66 17.66l1.41 1.41M2 12h2M20 12h2M6.34 17.66l-1.41 1.41M19.07 4.93l-1.41 1.41" />
    </svg>
  );
}

export default function Layout() {
  const theme = useThemeStore((s) => s.theme);
  const toggle = useThemeStore((s) => s.toggle);
  const username = useAuthStore((s) => s.username);
  const logout = useAuthStore((s) => s.logout);
  const navigate = useNavigate();

  function handleLogout() {
    logout();
    navigate("/login", { replace: true });
  }

  return (
    <div className="flex flex-1 flex-col">
      <header className="flex flex-nowrap items-center gap-6 border-b border-border bg-surface px-5 py-2.5 max-[600px]:flex-wrap max-[600px]:gap-x-3 max-[600px]:gap-y-2">
        <span className="font-semibold">HermiNas</span>
        <nav className="flex flex-1 gap-4 max-[600px]:order-3 max-[600px]:basis-full">
          <NavLink
            to="/query"
            className={({ isActive }) =>
              `no-underline transition-colors duration-150 ${isActive ? "font-semibold text-accent" : "text-muted-fg"}`
            }
          >
            Query Studio
          </NavLink>
        </nav>
        <div className="flex items-center gap-2.5">
          <button onClick={toggle} className="btn" aria-label="Toggle color theme" title="Toggle color theme">
            <ThemeIcon theme={theme} />
          </button>
          {username && <span className="text-[13px] text-muted-fg">{username}</span>}
          {username && (
            <button onClick={handleLogout} className="btn" aria-label="Sign out">
              Sign out
            </button>
          )}
        </div>
      </header>
      <main className="flex-1 p-5">
        <Outlet />
      </main>
    </div>
  );
}
