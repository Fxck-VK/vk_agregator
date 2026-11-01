import { describe, expect, it } from "vitest";
import { renderToStaticMarkup } from "react-dom/server";
import { UserConsole, usersPath } from "./UsersReferralsAuditScreens";
import type { OperatorUsersDTO } from "../api/usersReferralsAudit";

describe("account public ID lookup", () => {
  it("routes support IDs to the account lookup and retains legacy UUID lookup", () => {
    expect(usersPath({ userId: " ep-000123 " })).toBe("/admin/users/operator?public_id=EP-000123");
    expect(usersPath({ userId: "10000000-0000-4000-8000-000000000001" })).toBe("/admin/users/operator?user_id=10000000-0000-4000-8000-000000000001");
    expect(usersPath({ userId: " " })).toBe("/admin/users/operator");
  });
  it("shows web-only account results without requiring a VK user", () => {
    const data: OperatorUsersDTO = {
      generated_at: "2026-10-31T00:00:00Z",
      account: { public_id: "EP-000123", account_ref: "account:safe", status: "active", role: "user", account_type: "personal", locale: "ru", created_at: "2026-10-30T00:00:00Z", updated_at: "2026-10-31T00:00:00Z" },
      payment: { status: "not_loaded", total: 0, pending: 0, succeeded: 0, failed: 0, refunded: 0, credits_purchased: 0 },
      referrals: { status: "not_loaded", invited: 0, registered: 0, activated: 0, rewarded: 0 },
    };
    const html = renderToStaticMarkup(<UserConsole data={data} loading={false} />);
    expect(html).toContain("EP-000123");
    expect(html).toContain("account:safe");
    expect(html).toContain("active");
    expect(html).not.toContain("User not selected");
    expect(html).not.toContain("Jobs and payments");
  });
});
