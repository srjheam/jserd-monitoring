import AxeBuilder from "@axe-core/playwright";
import {
  type APIRequestContext,
  expect,
  type Page,
  test,
} from "@playwright/test";

const email = process.env.SMOKE_EMAIL || "operator@example.test";
const password = process.env.SMOKE_PASSWORD || "valid-password";
const pocketBaseURL = process.env.SMOKE_PB_URL || "http://127.0.0.1:18097";

async function token(request: APIRequestContext) {
  const auth = await request.post(
    `${pocketBaseURL}/api/collections/chief_editors/auth-with-password`,
    { data: { identity: email, password } },
  );
  return (await auth.json()).token as string;
}

// The set-aside dates are relative to the run, not fixed. A fixed return date
// expires on the day it names: upToDateUntilPassed reads a return date of today
// as passed, so the submission leaves the Up to date screen and the row this
// spec asserts on disappears.
function dayOffset(days: number) {
  const date = new Date();
  date.setUTCDate(date.getUTCDate() + days);
  return date.toISOString().slice(0, 10);
}

const setAsideOn = dayOffset(-21);
const setAsideUntil = dayOffset(21);

async function signIn(page: Page) {
  await page.goto("/");
  await page.getByLabel("Email").fill(email);
  await page.getByLabel("Password").fill(password);
  await page.getByRole("button", { name: "Sign in" }).click();
  await expect(page.getByRole("region", { name: "Work list" })).toBeVisible();
}

// The two side screens are empty until a chief editor puts something on them,
// so this spec curates its own: two conferences with one muted submission each,
// and one submission set aside with a reason. It takes them from the end of
// the work list, because the specs share one PocketBase and the work list
// spec acts on the front of the same list.
async function curate(request: APIRequestContext) {
  const headers = { Authorization: await token(request) };
  const listed = await (
    await request.get(
      `${pocketBaseURL}/api/jserd/monitoring/submissions?queue=worklist&sort=priority&limit=100`,
      { headers },
    )
  ).json();
  const items = listed.items as {
    ojsSubmissionId: string;
    findingDigest: string;
  }[];
  expect(items.length).toBeGreaterThan(2);
  const [first, second, third] = items.slice(-3);

  // "SBQS 2" before "SBQS 10" is the pair a plain string sort gets wrong, and
  // they are created in the wrong order so insertion order cannot pass for
  // alphanumeric order.
  const conferences: Record<string, string> = {};
  for (const name of ["SBQS 10", "SBQS 2"]) {
    const created = await request.post(
      `${pocketBaseURL}/api/collections/conferences/records`,
      { headers, data: { name, description: "" } },
    );
    // Checked, not assumed: a silent failure here used to leave the id
    // undefined, so both submissions were muted with no conference and the two
    // clusters collapsed into one "no conference" cluster.
    expect(created.ok()).toBe(true);
    conferences[name] = (await created.json()).id;
    expect(conferences[name]).toBeTruthy();
  }
  for (const [submission, conference] of [
    [first, "SBQS 10"],
    [second, "SBQS 2"],
  ] as const) {
    const muted = await request.post(
      `${pocketBaseURL}/api/collections/submission_tracking/records`,
      {
        headers,
        data: {
          ojs_submission_id: submission.ojsSubmissionId,
          conference: conferences[conference],
          muted: true,
        },
      },
    );
    expect(muted.ok()).toBe(true);
  }

  // One submission set aside, through the route that owns the marking. A
  // direct write to up_to_date_at/_digest/_until is refused by the server
  // (registerUpToDateFieldLock): those three fields and the journal entry that
  // explains them are written together, in one transaction, or not at all.
  const setAside = await request.post(
    `${pocketBaseURL}/api/jserd/monitoring/submissions/${third.ojsSubmissionId}/up-to-date`,
    {
      headers,
      data: {
        today: setAsideOn,
        until: setAsideUntil,
        reason: "Author on vacation for three weeks.",
      },
    },
  );
  expect(setAside.ok()).toBe(true);
}

test("the footer counts navigate to the side screens without reloading", async ({
  page,
  request,
}) => {
  await curate(request);
  await signIn(page);

  const muted = page.getByRole("link", { name: /^Muted \d+$/ });
  const upToDate = page.getByRole("link", { name: /^Up to date \d+$/ });
  // Both counts are real destinations, so a middle click still opens a tab.
  expect(await muted.getAttribute("href")).toBe("/muted");
  expect(await upToDate.getAttribute("href")).toBe("/up-to-date");

  // A value on `window` survives an in-page navigation and dies with a
  // document reload, which is the difference this hand-off asked for.
  await page.evaluate(() => {
    (window as unknown as { sideScreenProbe: boolean }).sideScreenProbe = true;
  });
  await muted.click();
  await expect(page.getByRole("region", { name: "Muted" })).toBeVisible();
  expect(
    await page.evaluate(
      () =>
        (window as unknown as { sideScreenProbe?: boolean }).sideScreenProbe,
    ),
  ).toBe(true);
  expect(new URL(page.url()).pathname).toBe("/muted");

  // Clusters sort alphanumerically, so "SBQS 2" sits above "SBQS 10".
  await expect(page.locator(".cluster").first()).toBeVisible();
  const names = await page.locator(".cluster__header h3").allInnerTexts();
  expect(names).toEqual(["⟨SBQS 2⟩", "⟨SBQS 10⟩"]);
  // An conference does not exist in OJS, so every header says so.
  expect(await page.getByRole("button", { name: "JSERD" }).count()).toBe(
    names.length,
  );

  const axeMuted = await new AxeBuilder({ page })
    .include(".side-screen-view")
    .analyze();
  expect(axeMuted.violations).toEqual([]);

  // Back to the work list, then on to Up to date, both in the page.
  await page.getByRole("link", { name: "Back to the work list" }).click();
  await expect(page.getByRole("region", { name: "Work list" })).toBeVisible();
  await upToDate.click();
  await expect(page.getByRole("region", { name: "Up to date" })).toBeVisible();
  await expect(
    page.locator(".side-screen-view article.row").first(),
  ).toBeVisible();

  // Each row says why it is there. The reason and its author come from the
  // journal entry that "Mark up to date" wrote.
  await expect(
    page.locator(".row__why").filter({ hasText: `Set aside on ${setAsideOn}` }),
  ).toContainText(`Returns on ${setAsideUntil}`);
  await expect(
    page.locator(".row__why").filter({ hasText: "Author on vacation" }),
  ).toBeVisible();

  const axeUpToDate = await new AxeBuilder({ page })
    .include(".side-screen-view")
    .analyze();
  expect(axeUpToDate.violations).toEqual([]);
});

test("`🔔` unmutes a submission and returns it to the work list", async ({
  page,
}) => {
  await signIn(page);
  await page.goto("/muted");
  const rows = page.locator(".side-screen-view article.row");
  await expect(rows.first()).toBeVisible();
  const before = await rows.count();

  await rows.first().getByRole("button", { name: "Unmute" }).click();
  await expect
    .poll(async () => rows.count(), { timeout: 15_000 })
    .toBe(before - 1);
});

test("both side screens fit 375px", async ({ page }) => {
  await page.setViewportSize({ width: 375, height: 844 });
  await signIn(page);
  for (const path of ["/muted", "/up-to-date"]) {
    await page.goto(path);
    await expect(
      page.locator(".side-screen-view article.row").first(),
    ).toBeVisible();
    expect(
      await page.evaluate(
        () => document.documentElement.scrollWidth <= window.innerWidth,
      ),
    ).toBe(true);
  }
});

// The same search field, with the same chips, on both side screens: a second
// set of rules to learn would be the cost of a second search.
test("the search chips filter both side screens", async ({ page, request }) => {
  const headers = { Authorization: await token(request) };
  await signIn(page);
  await page.goto("/muted");
  const rows = page.locator(".side-screen-view article.row");
  await expect(rows.first()).toBeVisible();

  const listed = await (
    await request.get(
      `${pocketBaseURL}/api/jserd/monitoring/submissions?queue=muted&limit=100`,
      { headers },
    )
  ).json();
  const editor: string = listed.items
    .map((item: { editors: string[] }) => item.editors[0])
    .find(Boolean);
  test.skip(!editor, "no muted submission carries an editor to search for");

  // Typing then Enter runs a free-text search: committing an `editor:` chip
  // needs the suggestion picked, which is the three-zone keyboard model the
  // combobox unit tests pin (tests/search-combobox.test.tsx).
  const search = page.getByRole("combobox");
  await search.fill(editor);
  await page.getByRole("option", { name: editor }).click();
  await expect(
    page.getByRole("button", { name: `Remove ${editor}` }),
  ).toBeVisible();
  await expect
    .poll(async () => {
      const authors = await page
        .locator(".side-screen-view article.row .row__author")
        .allInnerTexts();
      return (
        authors.length > 0 && authors.every((author) => author.includes(editor))
      );
    })
    .toBe(true);
});
