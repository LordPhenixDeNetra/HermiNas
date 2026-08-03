import { beforeEach, describe, expect, it } from "vitest";
import { render, screen } from "@testing-library/react";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import ProtectedRoute from "./ProtectedRoute";
import { useAuthStore } from "../store/auth";

function renderAt(path: string) {
  render(
    <MemoryRouter initialEntries={[path]}>
      <Routes>
        <Route path="/login" element={<div>login page</div>} />
        <Route element={<ProtectedRoute />}>
          <Route path="/query" element={<div>query studio</div>} />
        </Route>
      </Routes>
    </MemoryRouter>,
  );
}

describe("ProtectedRoute", () => {
  beforeEach(() => {
    useAuthStore.getState().logout();
  });

  it("redirects to /login when unauthenticated", () => {
    renderAt("/query");
    expect(screen.getByText("login page")).toBeInTheDocument();
  });

  it("renders the protected content when authenticated", () => {
    useAuthStore.getState().setSession("token-123", "alice", "engineer");
    renderAt("/query");
    expect(screen.getByText("query studio")).toBeInTheDocument();
  });
});
