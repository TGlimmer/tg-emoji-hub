import { create } from "zustand";
import type { EmojiSticker } from "@/api/types";

export type SearchStatus = "idle" | "loading" | "success" | "error" | "empty";

interface SearchState {
  query: string;
  results: EmojiSticker[];
  total: number;
  status: SearchStatus;
  errorMessage: string;
  // 当前筛选
  packFilter: string;
  categoryFilter: string;
  animatedOnly: boolean;
  // 键盘焦点索引
  focusedIndex: number;

  setQuery: (q: string) => void;
  setPackFilter: (p: string) => void;
  setCategoryFilter: (c: string) => void;
  setAnimatedOnly: (v: boolean) => void;
  setFocusedIndex: (i: number) => void;
  setResults: (items: EmojiSticker[], total: number) => void;
  setStatus: (s: SearchStatus, errorMessage?: string) => void;
  reset: () => void;
}

export const useSearchStore = create<SearchState>((set) => ({
  query: "",
  results: [],
  total: 0,
  status: "idle",
  errorMessage: "",
  packFilter: "",
  categoryFilter: "",
  animatedOnly: false,
  focusedIndex: -1,

  setQuery: (q) => set({ query: q }),
  setPackFilter: (p) => set({ packFilter: p, focusedIndex: -1 }),
  setCategoryFilter: (c) => set({ categoryFilter: c, focusedIndex: -1 }),
  setAnimatedOnly: (v) => set({ animatedOnly: v, focusedIndex: -1 }),
  setFocusedIndex: (i) => set({ focusedIndex: i }),
  setResults: (items, total) =>
    set({
      results: items,
      total,
      status: items.length === 0 ? "empty" : "success",
      errorMessage: "",
      focusedIndex: -1,
    }),
  setStatus: (s, errorMessage = "") => set({ status: s, errorMessage }),
  reset: () =>
    set({
      results: [],
      total: 0,
      status: "idle",
      errorMessage: "",
      focusedIndex: -1,
    }),
}));
