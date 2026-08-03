import { beforeEach, describe, expect, it } from "vitest";
import { useAuthStore, selectIsAuthenticated } from "./auth";

describe("auth store", () => {
  beforeEach(() => {
    useAuthStore.getState().logout();
    localStorage.clear();
  });

  it("starts unauthenticated", () => {
    expect(selectIsAuthenticated(useAuthStore.getState())).toBe(false);
  });

  it("becomes authenticated after setSession", () => {
    useAuthStore.getState().setSession("token-123", "alice", "engineer");
    const state = useAuthStore.getState();
    expect(selectIsAuthenticated(state)).toBe(true);
    expect(state.token).toBe("token-123");
    expect(state.username).toBe("alice");
    expect(state.role).toBe("engineer");
  });

  it("clears everything on logout", () => {
    useAuthStore.getState().setSession("token-123", "alice", "engineer");
    useAuthStore.getState().logout();
    const state = useAuthStore.getState();
    expect(selectIsAuthenticated(state)).toBe(false);
    expect(state.token).toBeNull();
    expect(state.username).toBeNull();
  });

  it("persists the session to localStorage", () => {
    useAuthStore.getState().setSession("token-123", "alice", "engineer");
    const raw = localStorage.getItem("herminas-auth");
    expect(raw).not.toBeNull();
    expect(JSON.parse(raw!).state.token).toBe("token-123");
  });
});
