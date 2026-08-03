import { NavLink, Outlet, useNavigate } from "react-router-dom";
import { useThemeStore } from "../store/theme";
import { useAuthStore } from "../store/auth";

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
    <div className="app-shell">
      <header className="app-header">
        <span className="brand">HermiNas</span>
        <nav>
          <NavLink to="/query">Query Studio</NavLink>
        </nav>
        <div className="header-right">
          <button onClick={toggle} aria-label="Toggle color theme" title="Toggle color theme">
            {theme === "light" ? "🌙" : "☀️"}
          </button>
          {username && <span className="username">{username}</span>}
          {username && (
            <button onClick={handleLogout} aria-label="Sign out">
              Sign out
            </button>
          )}
        </div>
      </header>
      <main>
        <Outlet />
      </main>
    </div>
  );
}
