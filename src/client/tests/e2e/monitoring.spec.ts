import AxeBuilder from "@axe-core/playwright";
import { expect, test } from "@playwright/test";

const email = process.env.SMOKE_EMAIL || "operator@example.test";
const password = process.env.SMOKE_PASSWORD || "valid-password";
const pocketBaseURL = process.env.SMOKE_PB_URL || "http://127.0.0.1:18097";

test("JSERD monitoring baseline flow", async ({ page, request }) => {
  const origins = new Set<string>();
  const pageErrors: string[] = [];
  page.on("pageerror", (error) => pageErrors.push(error.message));
  page.on("request", (request) => origins.add(new URL(request.url()).origin));
  await page.goto("/");
  await page.getByLabel("Email").fill(email);
  await page.getByLabel("Password").fill(password);
  await page.getByRole("button", { name: "Sign in" }).click();
  await expect(
    page.getByRole("heading", { name: "Editorial monitoring" }),
  ).toBeVisible();
  await expect(page.getByText(/^Last sync with OJS: /)).toBeVisible();
  await expect(page.getByRole("button", { name: "Refresh" })).toHaveCount(0);
  await expect(page.getByRole("button", { name: /Language/ })).toBeVisible();
  await expect(page.getByRole("button", { name: /Account/ })).toBeVisible();
  try {
    await expect(page.getByRole("region", { name: "Work list" })).toBeVisible();
  } catch {
    throw new Error(`Dashboard failed to render: ${pageErrors.join(" | ")}`);
  }

  // A full-page batch keeps the sentinel below the fold so the "Load more"
  // path is verified deterministically. `limit` is only an entry parameter,
  // so a reload drops it and always restarts at the first default-size batch.
  await page.goto("/?limit=25");
  await expect(page.locator("article.row")).toHaveCount(25);
  await page.getByRole("button", { name: "Load more" }).click();
  await expect
    .poll(async () => page.locator("article.row").count())
    .toBeGreaterThan(25);
  const firstRowHref = await page
    .locator("article.row .row__link")
    .first()
    .getAttribute("href");
  await page.reload();
  await expect(page.locator("article.row")).toHaveCount(25);
  await expect(page.locator("article.row .row__link").first()).toHaveAttribute(
    "href",
    firstRowHref ?? "",
  );

  // A tiny batch keeps the sentinel on screen: rows append automatically
  // until the filled viewport pushes it out of the trigger margin.
  await page.goto("/?limit=2");
  await expect
    .poll(async () => page.locator("article.row").count(), { timeout: 15_000 })
    .toBeGreaterThan(2);

  await page.goto("/");
  const submissionID = new URL(
    (await page
      .locator("article.row .row__link")
      .first()
      .getAttribute("href")) ?? "",
    "http://localhost",
  ).searchParams.get("article");
  expect(submissionID).toBeTruthy();
  const axe = await new AxeBuilder({ page }).analyze();
  expect(
    axe.violations.filter((violation) =>
      ["serious", "critical"].includes(violation.impact || ""),
    ),
  ).toEqual([]);

  // The title is the only control that navigates, and it is a real anchor.
  await page.locator("article.row").first().getByRole("link").first().click();
  await expect(
    page.getByRole("heading", { name: `${submissionID} Open in OJS` }),
  ).toBeVisible();
  await expect
    .poll(() => new URL(page.url()).searchParams.get("article"))
    .toBe(submissionID);
  const sharedArticleURL = page.url();
  await page.goto(sharedArticleURL);
  await expect(
    page.getByRole("heading", { name: `${submissionID} Open in OJS` }),
  ).toBeVisible();
  const panelAxe = await new AxeBuilder({ page }).analyze();
  expect(
    panelAxe.violations.filter((violation) =>
      ["serious", "critical"].includes(violation.impact || ""),
    ),
  ).toEqual([]);
  await page.keyboard.press("Escape");
  await expect(
    page.getByRole("heading", { name: `${submissionID} Open in OJS` }),
  ).not.toBeVisible();
  await page.goto(sharedArticleURL);
  await expect(
    page.getByRole("heading", { name: `${submissionID} Open in OJS` }),
  ).toBeVisible();

  const hamburger = page.getByRole("button", { name: "Menu" });
  await hamburger.focus();
  await expect(hamburger).toBeFocused();
  await hamburger.press("Enter");
  await expect(
    page.getByRole("menuitem", { name: "Manage conferences" }),
  ).toBeFocused();
  await page.keyboard.press("Enter");
  await expect(
    page.getByRole("heading", { name: "Conferences" }),
  ).toBeVisible();
  await page.keyboard.press("Escape");
  await expect(
    page.getByRole("heading", { name: "Conferences" }),
  ).not.toBeVisible();
  await expect(hamburger).toBeFocused();

  const editorAuth = await request.post(
    `${pocketBaseURL}/api/collections/chief_editors/auth-with-password`,
    { data: { identity: email, password } },
  );
  const editorToken = (await editorAuth.json()).token;
  const superuserAuth = await request.post(
    `${pocketBaseURL}/api/collections/_superusers/auth-with-password`,
    { data: { identity: email, password } },
  );
  const superuserToken = (await superuserAuth.json()).token;
  const beforeCycle = await request.get(
    `${pocketBaseURL}/api/jserd/monitoring`,
    {
      headers: { Authorization: editorToken },
    },
  );
  const previousAttemptID = (await beforeCycle.json()).latestAttempt?.id;
  await request.post(
    `${pocketBaseURL}/api/collections/manual_collection_requests/records`,
    { headers: { Authorization: superuserToken }, data: { confirmed: true } },
  );
  await expect
    .poll(
      async () => {
        const response = await request.get(
          `${pocketBaseURL}/api/jserd/monitoring`,
          { headers: { Authorization: editorToken } },
        );
        const projection = await response.json();
        return projection.latestAttempt?.id !== previousAttemptID &&
          projection.latestAttempt?.status === "succeeded"
          ? "succeeded"
          : "pending";
      },
      { timeout: 30_000 },
    )
    .toBe("succeeded");
  await page.reload();
  await expect(
    page.getByRole("heading", { name: `${submissionID} Open in OJS` }),
  ).toBeVisible();
  for (const width of [375, 768, 1024, 1440]) {
    await page.setViewportSize({ width, height: 844 });
    await expect(
      page.getByRole("heading", { name: `${submissionID} Open in OJS` }),
    ).toBeVisible();
    expect({
      width,
      fits: await page.evaluate(
        () => document.documentElement.scrollWidth <= window.innerWidth,
      ),
    }).toEqual({ width, fits: true });
  }
  await page.setViewportSize({ width: 375, height: 844 });
  await expect(page.getByRole("button", { name: "Menu" })).toBeVisible();
  await expect(page.getByRole("button", { name: /Language/ })).toHaveCount(0);
  await page.screenshot({
    path: "/tmp/jserd-final-ux.png",
    fullPage: true,
  });
  for (const origin of origins)
    expect([new URL(page.url()).origin, pocketBaseURL]).toContain(origin);
});
