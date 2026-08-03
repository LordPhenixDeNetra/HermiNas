import { create } from "zustand";
import { persist } from "zustand/middleware";

interface AuthState {
  token: string | null;
  username: string | null;
  role: string | null;
  setSession: (token: string, username: string, role: string) => void;
  logout: () => void;
}

// Persisted to localStorage under "herminas-auth" so a page refresh
// doesn't force a re-login — the JWT itself (M1.5) still expires
// server-side after its own TTL regardless of what's cached here.
export const useAuthStore = create<AuthState>()(
  persist(
    (set) => ({
      token: null,
      username: null,
      role: null,
      setSession: (token, username, role) => set({ token, username, role }),
      logout: () => set({ token: null, username: null, role: null }),
    }),
    { name: "herminas-auth" },
  ),
);

export function selectIsAuthenticated(state: AuthState): boolean {
  return state.token !== null;
}
