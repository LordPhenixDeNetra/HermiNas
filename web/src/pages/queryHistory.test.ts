import { beforeEach, describe, expect, it } from "vitest";
import { loadHistory, pushHistory } from "./queryHistory";

describe("queryHistory", () => {
  beforeEach(() => {
    localStorage.clear();
  });

  it("starts empty", () => {
    expect(loadHistory()).toEqual([]);
  });

  it("pushHistory adds the newest query to the front", () => {
    let history = pushHistory([], "SELECT 1");
    history = pushHistory(history, "SELECT 2");
    expect(history).toEqual(["SELECT 2", "SELECT 1"]);
  });

  it("re-running the same query moves it to the front instead of duplicating it", () => {
    let history = pushHistory([], "SELECT 1");
    history = pushHistory(history, "SELECT 2");
    history = pushHistory(history, "SELECT 1");
    expect(history).toEqual(["SELECT 1", "SELECT 2"]);
  });

  it("persists across loadHistory calls", () => {
    pushHistory([], "SELECT 1");
    expect(loadHistory()).toEqual(["SELECT 1"]);
  });

  it("caps history length at 20 entries", () => {
    let history: string[] = [];
    for (let i = 0; i < 25; i++) {
      history = pushHistory(history, `SELECT ${i}`);
    }
    expect(history).toHaveLength(20);
    expect(history[0]).toBe("SELECT 24");
  });

  it("loadHistory tolerates corrupted storage", () => {
    localStorage.setItem("herminas-query-history", "not json");
    expect(loadHistory()).toEqual([]);
  });
});
