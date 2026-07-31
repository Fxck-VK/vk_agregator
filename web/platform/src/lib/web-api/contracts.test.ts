import { describe, expect, it } from "vitest";

import { parseAccountProfile, parseConversationList, publicApiErrorSchema } from "./contracts";

describe("AccountProfile contract", () => {
  it("accepts only the documented safe profile fields", () => {
    expect(
      parseAccountProfile({
        account_id: "62d33e7f-7b0e-4a26-975b-41080b55d78d",
        identity_refs: [
          {
            id: "d7c979f5-24e5-4f88-924b-a592d6e5a906",
            account_id: "62d33e7f-7b0e-4a26-975b-41080b55d78d",
            provider: "email",
            label: "a***@example.com",
            verified: true,
            created_at: "2026-07-31T09:00:00Z",
          },
        ],
      }),
    ).toEqual({
      account_id: "62d33e7f-7b0e-4a26-975b-41080b55d78d",
      identity_refs: [
        {
          id: "d7c979f5-24e5-4f88-924b-a592d6e5a906",
          account_id: "62d33e7f-7b0e-4a26-975b-41080b55d78d",
          provider: "email",
          label: "a***@example.com",
          verified: true,
          created_at: "2026-07-31T09:00:00Z",
        },
      ],
    });
  });

  it("does not accept speculative personal fields", () => {
    expect(() =>
      parseAccountProfile({
        account_id: "62d33e7f-7b0e-4a26-975b-41080b55d78d",
        identity_refs: [],
        avatar_url: "https://private.example/avatar.png",
      }),
    ).toThrow();
  });
});

describe("public API error contract", () => {
  it("trims the backend's public error text", () => {
    expect(publicApiErrorSchema.parse({ error: " unauthorized " })).toEqual({
      error: "unauthorized",
    });
  });
});

describe("Conversation list contract", () => {
  const item = {
    id: "d7c979f5-24e5-4f88-924b-a592d6e5a906",
    title: "New conversation",
    created_at: "2026-07-31T09:00:00Z",
    updated_at: "2026-07-31T09:05:00Z",
  };

  it("accepts only the documented safe conversation fields", () => {
    expect(parseConversationList({ items: [item] })).toEqual({ items: [item] });
  });

  it.each(["account_id", "source"])("rejects a conversation item with %s", (field) => {
    expect(() => parseConversationList({ items: [{ ...item, [field]: "forged" }] })).toThrow();
  });
});
