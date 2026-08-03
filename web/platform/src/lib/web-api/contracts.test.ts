import { describe, expect, it } from "vitest";

import {
  parseAccountProfile,
  parseAccountBalance,
  parseConversationList,
  parseImageJobPreparation,
  parseImageJobList,
  parseImageModelList,
  publicApiErrorSchema,
} from "./contracts";

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

describe("Account balance contract", () => {
  it("accepts only an account-scoped credit balance", () => {
    expect(parseAccountBalance({ balance: 104 })).toEqual({ balance: 104 });
    expect(() => parseAccountBalance({ balance: 104, account_id: "private" })).toThrow();
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

describe("Image generation contracts", () => {
  const imageModel = {
    id: "nano-banana-2",
    name: "Nano Banana 2",
    quality_options: ["1K", "2K"],
    price_by_quality: { "1K": 16, "2K": 60 },
    default_quality: "1K",
    supports_reference_image: true,
    max_reference_images: 4,
  };

  const imageJob = {
    id: "d7c979f5-24e5-4f88-924b-a592d6e5a906",
    status: "prepared",
    prompt: "night city after rain",
    model_id: "nano-banana-2",
    model_name: "Nano Banana 2",
    image_quality: "2K",
    cost_estimate: 60,
    created_at: "2026-08-01T12:00:00Z",
    updated_at: "2026-08-01T12:00:00Z",
  };

  it("accepts the explicitly safe image model and preparation payloads", () => {
    expect(parseImageModelList({ items: [imageModel] })).toEqual({ items: [imageModel] });
    expect(
      parseImageJobPreparation({
        job: imageJob,
        balance: 104,
        can_afford: true,
      }),
    ).toEqual({
      job: imageJob,
      balance: 104,
      can_afford: true,
    });
  });

  it("accepts only positive public prices for a model quality", () => {
    expect(() => parseImageModelList({
      items: [{ ...imageModel, price_by_quality: { "1K": 0 } }],
    })).toThrow();
  });

  it.each(["provider", "model_code", "pricing_snapshot", "storage_key"])(
    "rejects a browser response that exposes %s",
    (privateField) => {
      expect(() => parseImageJobPreparation({ job: { ...imageJob, [privateField]: "private" }, balance: 104, can_afford: true })).toThrow();
    },
  );

  it("accepts an opaque cursor for the next bounded image history page", () => {
    const page = {
      items: [imageJob],
      has_more: true,
      next_cursor: "eyJjcmVhdGVkX2F0IjoiMjAyNi0wOC0wMVQxMjowMDowMFoiLCJpZCI6ImQ3Yzk3ZjUtMjRlNS00Zjg4LTkyNGItYTU5MmQ2ZTVhOTA2In0",
    };

    expect(parseImageJobList(page)).toEqual(page);
    expect(() => parseImageJobList({ ...page, storage_url: "https://objects.example.test/private" })).toThrow();
  });
});
