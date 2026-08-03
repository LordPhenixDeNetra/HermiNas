import { useState, type FormEvent } from "react";
import { useNavigate } from "react-router-dom";
import { login, ApiError } from "../api/client";
import { useAuthStore } from "../store/auth";

export default function Login() {
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);
  const setSession = useAuthStore((s) => s.setSession);
  const navigate = useNavigate();

  async function handleSubmit(e: FormEvent) {
    e.preventDefault();
    setError(null);
    setLoading(true);
    try {
      const resp = await login(username, password);
      setSession(resp.token, username, resp.role);
      navigate("/query", { replace: true });
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Login failed");
    } finally {
      setLoading(false);
    }
  }

  return (
    <div className="flex flex-1 items-center justify-center">
      <form onSubmit={handleSubmit} className="flex w-80 flex-col gap-3 rounded-xl border border-border bg-surface p-8">
        <h1 className="mb-2 text-[22px]">HermiNas</h1>
        <label className="flex flex-col gap-1 text-[13px] text-muted-fg">
          Username
          <input
            className="input"
            value={username}
            onChange={(e) => setUsername(e.target.value)}
            autoComplete="username"
            required
          />
        </label>
        <label className="flex flex-col gap-1 text-[13px] text-muted-fg">
          Password
          <input
            className="input"
            type="password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            autoComplete="current-password"
            required
          />
        </label>
        {error && (
          <p role="alert" className="rounded-md bg-danger-bg px-3 py-2 text-danger">
            {error}
          </p>
        )}
        <button type="submit" className="btn btn-primary" disabled={loading}>
          {loading ? "Signing in…" : "Sign in"}
        </button>
      </form>
    </div>
  );
}
