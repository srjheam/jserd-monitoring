import { expect, test } from "@playwright/test";

const email = process.env.SMOKE_EMAIL || "operator@example.test";
const password = process.env.SMOKE_PASSWORD || "valid-password";
const pocketBaseURL = process.env.SMOKE_PB_URL || "http://127.0.0.1:18097";
const occurredAt = "2025-05-21T09:00:00Z";

test("mail activity stays readable and localized across viewports", async ({
  page,
  request,
}) => {
  const auth = await request.post(
    `${pocketBaseURL}/api/collections/chief_editors/auth-with-password`,
    { data: { identity: email, password } },
  );
  const token = (await auth.json()).token;
  const listed = await request.get(
    `${pocketBaseURL}/api/jserd/monitoring/submissions?limit=1`,
    { headers: { Authorization: token } },
  );
  const submissionID = (await listed.json()).items[0].ojsSubmissionId as string;
  expect(submissionID).toBeTruthy();

  await page.route("**/api/jserd/monitoring/submissions/*/activity", (route) =>
    route.fulfill({
      json: [
        {
          kind: "delivery",
          occurredAt,
          titleKey: "review_invitation_answered",
          recipientRole: "editor",
          recipient: "editor@example.test",
          status: "sent",
        },
        {
          kind: "delivery",
          occurredAt,
          titleKey: "review_deadline_expired",
          recipientRole: "reviewer",
          recipient: "reviewer@example.test",
          status: "failed",
        },
        {
          kind: "journal",
          occurredAt,
          body: "Journal layout check",
          authorLabel: "Chief editor",
        },
      ],
    }),
  );

  await page.goto("/");
  await page.getByLabel("Email").fill(email);
  await page.getByLabel("Password").fill(password);
  await page.getByRole("button", { name: "Sign in" }).click();
  await expect(
    page.getByRole("heading", { name: "Editorial monitoring" }),
  ).toBeVisible();
  await page.goto(`/?article=${encodeURIComponent(submissionID)}`);

  const entries = page.locator(".activity-entry--mail");
  await expect(entries).toHaveCount(2);
  const invitation = entries.nth(0);
  const deadline = entries.nth(1);
  const journal = page
    .locator(".activity-entry")
    .filter({ hasText: "Journal layout check" });
  const invitationTitle = invitation.locator(
    ".activity-entry-detail > span:first-child",
  );
  const expectedDate = await page.evaluate(
    (date) =>
      new Intl.DateTimeFormat(navigator.language, {
        dateStyle: "medium",
      }).format(new Date(date)),
    occurredAt,
  );
  await expect(invitation.locator(".activity-when")).toHaveText(expectedDate);
  await expect(journal.locator(".activity-entry-detail")).toContainText(
    "Journal layout check",
  );
  await expect(journal.locator(".activity-entry-detail")).toContainText(
    "Chief editor",
  );
  await expect(
    journal.locator(".activity-entry-detail .activity-when"),
  ).toHaveText(expectedDate);
  await expect(journal.locator(".lucide-pencil")).toHaveCount(1);
  await expect(invitation).toContainText(
    "To editor@example.test · Assigned editor",
  );
  await expect(deadline).toContainText("To reviewer@example.test · Reviewer");
  await expect(invitation).toContainText("The review invitation was answered.");
  await expect(deadline).toContainText(
    "The deadline to submit the review has passed.",
  );

  await page.setViewportSize({ width: 1280, height: 900 });
  const desktopGap = await page.evaluate(() => {
    const iconRect = document
      .querySelector(".activity-entry--mail .activity-mark")
      ?.getBoundingClientRect();
    const titleRect = document
      .querySelector(".activity-entry--mail .activity-entry-detail > span")
      ?.getBoundingClientRect();
    if (!iconRect || !titleRect) return null;
    return titleRect.left - iconRect.right;
  });
  expect(desktopGap).toBeGreaterThan(0);

  await expect(invitationTitle).toHaveCSS("text-decoration-line", "none");
  await expect(invitation.locator(".lucide-arrow-up-right")).toHaveCount(0);
  await expect(invitation.locator(".lucide-mail-open")).toHaveCount(0);

  await page.reload();
  const refreshedIcon = page
    .locator(".activity-entry--mail")
    .first()
    .locator(".activity-mark");
  await expect(refreshedIcon).toBeVisible();
  await page.mouse.move(0, 0);
  await refreshedIcon.hover();
  await expect(page.getByRole("tooltip")).toHaveText("delivered");
  await page.mouse.move(0, 0);
  await page.keyboard.press("Tab");
  await refreshedIcon.focus();
  await expect(page.getByRole("tooltip")).toHaveText("delivered");
  await refreshedIcon.click();
  await expect(page.getByRole("dialog")).toHaveCount(0);

  await page.getByRole("button", { name: /Language/ }).click();
  await page.getByRole("menuitemradio", { name: "Português" }).click();
  const portugueseDate = await page.evaluate(
    (date) =>
      new Intl.DateTimeFormat("pt-BR", { dateStyle: "medium" }).format(
        new Date(date),
      ),
    occurredAt,
  );
  await expect(invitation.locator(".activity-when")).toHaveText(portugueseDate);
  await expect(invitation).toContainText(
    "O convite de revisão foi respondido.",
  );
  await expect(invitation).toContainText(
    "Para editor@example.test · Editor responsável",
  );
  await expect(deadline).toContainText(
    "O prazo para entregar a revisão venceu.",
  );
  await expect(deadline).toContainText("Para reviewer@example.test · Revisor");

  await page.setViewportSize({ width: 375, height: 844 });
  for (const entry of [invitation, deadline, journal]) {
    await expect(
      entry.locator(".activity-entry-detail > span:first-child"),
    ).toBeVisible();
    await expect(
      entry.locator(".activity-recipient, .activity-author"),
    ).toBeVisible();
    const gap = await entry.evaluate((node) => {
      const iconRect = node
        .querySelector(".activity-mark")
        ?.getBoundingClientRect();
      const titleRect = node
        .querySelector(".activity-entry-detail > span:first-child")
        ?.getBoundingClientRect();
      if (!iconRect || !titleRect) return null;
      return titleRect.left - iconRect.right;
    });
    expect(gap).toBeGreaterThan(0);
  }
});
