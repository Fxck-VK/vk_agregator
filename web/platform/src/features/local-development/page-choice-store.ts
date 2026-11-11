"use client";

export type DevelopmentPageChoice = {
  id: string;
  title: string;
  description?: string;
  value: string;
  options: readonly { id: string; label: string }[];
  onChange: (value: string) => void;
};

let current: { owner: symbol; choice: DevelopmentPageChoice } | null = null;
const listeners = new Set<() => void>();
const emit = () => listeners.forEach(listener => listener());

/** Nonpersistent presentation controls owned by the currently mounted DEV page. */
export const pageChoiceStore = {
  subscribe(listener: () => void) { listeners.add(listener); return () => { listeners.delete(listener); }; },
  getSnapshot: () => current?.choice ?? null,
  getServerSnapshot: () => null,
  register(choice: DevelopmentPageChoice) {
    const owner = Symbol(choice.id);
    current = { owner, choice }; emit();
    return () => { if (current?.owner === owner) { current = null; emit(); } };
  },
};
