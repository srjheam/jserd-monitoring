import AxeBuilder from "@axe-core/playwright";
import {
  type APIRequestContext,
  expect,
  type Locator,
  type Page,
  test,
} from "@playwright/test";

const email = process.env.SMOKE_EMAIL || "operator@example.test";
const password = process.env.SMOKE_PASSWORD || "valid-password";

async function signIn(page: Page) {
  await page.goto("/");
  await page.getByLabel("Email").fill(email);
  await page.getByLabel("Password").fill(password);
  await page.getByRole("button", { name: "Sign in" }).click();
  await expect(page.getByRole("region", { name: "Work list" })).toBeVisible();
}

async function token(request: APIRequestContext) {
  const auth = await request.post(
    `${process.env.SMOKE_PB_URL}/api/collections/chief_editors/auth-with-password`,
    { data: { identity: email, password } },
  );
  return (await auth.json()).token as string;
}

const box = async (locator: Locator) =>
  (await locator.boundingBox()) ?? { width: 0, height: 0 };

// The row id is read from the anchor href, which is the value the list acts
// on. `.row__id` also carries the hidden "ID" label OJS puts there.
const rowID = async (row: Locator) =>
  new URL(
    (await row.getByRole("link").first().getAttribute("href")) ?? "",
    "http://localhost",
  ).searchParams.get("article");

test("the work list is one list, keyboard reachable and accessible", async ({
  page,
  context,
}) => {
  await signIn(page);

  // One list, no tabs, and no `Ações` column.
  await expect(page.getByRole("grid")).toHaveCount(0);
  await expect(page.getByRole("table")).toHaveCount(0);
  for (const gone of [
    "Daily requiring attention",
    "Daily monitoring",
    "All active",
    "Event submissions",
    "Actions",
  ])
    await expect(page.getByRole("button", { name: gone })).toHaveCount(0);

  const rows = page.locator("article.row");
  await expect(rows.first()).toBeVisible();
  const rowCount = await rows.count();
  // A submission appears once. Every id in the list is distinct.
  const ids = await rows
    .locator(".row__link")
    .evaluateAll((nodes) =>
      nodes.map((node) => (node as HTMLAnchorElement).getAttribute("href")),
    );
  expect(new Set(ids).size).toBe(rowCount);

  const axe = await new AxeBuilder({ page }).include(".work-list").analyze();
  expect(axe.violations).toEqual([]);

  // Tab reaches the title, then `✎`, then the split button. The visible focus
  // ring never disappears on the way.
  const title = rows.first().getByRole("link").first();
  await title.focus();
  await expect(title).toBeFocused();
  const order = ["Add a note", "Mark up to date", "More up to date options"];
  for (const name of order) {
    await page.keyboard.press("Tab");
    const next = rows
      .first()
      .getByRole(name === "Add a note" ? "link" : "button", {
        name,
      });
    await expect(next).toBeFocused();
    expect(
      await next.evaluate((node) => getComputedStyle(node).outlineWidth),
    ).not.toBe("0px");
  }

  // Enter on the title navigates, and there is no row-level click handler.
  const firstID = await rowID(rows.first());
  await title.focus();
  await page.keyboard.press("Enter");
  await expect
    .poll(() => new URL(page.url()).searchParams.get("article"))
    .toBe(firstID);

  // Middle-click on a row opens a new tab, which a click handler would break.
  await page.goto("/");
  const target = page.locator("article.row").first().getByRole("link").first();
  const opened = context.waitForEvent("page");
  await target.click({ button: "middle" });
  const tab = await opened;
  expect(new URL(tab.url()).searchParams.get("article")).toBeTruthy();
  await tab.close();

  // `✎` is a link to the submission page with the note box focused, not a
  // popup: it carries `note=1`.
  const note = page.locator("article.row").first().getByRole("link", {
    name: "Add a note",
  });
  expect(await note.getAttribute("href")).toMatch(/\?article=[^&]+&note=1$/);
});

test("the work list fits 375px and keeps every control tappable", async ({
  page,
}) => {
  await page.setViewportSize({ width: 375, height: 844 });
  await signIn(page);
  await expect(page.locator("article.row").first()).toBeVisible();

  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= window.innerWidth,
    ),
  ).toBe(true);

  const row = page.locator("article.row").first();
  for (const control of [
    row.getByRole("link", { name: "Add a note" }),
    row.getByRole("button", { name: "Mark up to date" }),
    row.getByRole("button", { name: "More up to date options" }),
    page.getByRole("button", { name: "Clear" }),
    page.getByRole("combobox"),
  ]) {
    const size = await box(control);
    expect({ width: size.width, height: size.height }).toEqual({
      width: expect.any(Number),
      height: expect.any(Number),
    });
    expect(size.height).toBeGreaterThanOrEqual(44);
  }
});

test("search chips filter the list and the count sits at the bottom", async ({
  page,
  request,
}) => {
  await signIn(page);
  const footer = page.locator(".work-list__footer");
  await expect(footer).toContainText(/submissions? need(s)? you/);
  await expect(footer).toContainText(/Muted \d+/);
  await expect(footer).toContainText(/Up to date \d+/);
  // The count is a summary, so it comes after the rows it summarizes.
  expect(
    await page.evaluate(() => {
      const first = document.querySelector("article.row");
      const foot = document.querySelector(".work-list__footer");
      return Boolean(
        first &&
          foot &&
          first.compareDocumentPosition(foot) &
            Node.DOCUMENT_POSITION_FOLLOWING,
      );
    }),
  ).toBe(true);

  // `missing_editor` is rank 1, so the top of the list is exactly where no
  // editor name is. The editor to search for comes from the work list itself.
  const listed = await (
    await request.get(
      `${process.env.SMOKE_PB_URL}/api/jserd/monitoring/submissions?queue=worklist&sort=priority&limit=100`,
      { headers: { Authorization: await token(request) } },
    )
  ).json();
  const editor: string = listed.items
    .map((item: { editors: string[] }) => item.editors[0])
    .find(Boolean);
  expect(editor).toBeTruthy();

  // The footer total, not the rendered row count: both are capped at the page
  // limit, so a filter that still fills a batch would look like no filter at
  // all. The output reads "N submissions need you".
  const total = async () =>
    Number(
      (await page.locator(".work-list__footer output").innerText()).match(
        /\d+/,
      )?.[0],
    );
  const before = await total();
  // Typing then Enter runs a free-text search: committing an `editor:` chip
  // needs the suggestion picked, which is the three-zone keyboard model the
  // combobox unit tests pin (tests/search-combobox.test.tsx).
  const search = page.getByRole("combobox");
  await search.fill(editor);
  await page.getByRole("option", { name: editor }).click();
  await expect(
    page.getByRole("button", { name: `Remove ${editor}` }),
  ).toBeVisible();
  // The chip travels as `editor:"<name>"`, and every surviving row carries
  // that editor.
  await expect
    .poll(async () => {
      const authors = await page
        .locator("article.row .row__author")
        .allInnerTexts();
      return (
        authors.length > 0 && authors.every((author) => author.includes(editor))
      );
    })
    .toBe(true);
  await expect.poll(total).toBeLessThan(before);
});

test("marking up to date moves a submission off the list and writes one journal entry", async ({
  page,
  request,
}) => {
  await signIn(page);
  // More than one loaded batch, so a mark that removed a row by replaying the
  // keyset cursors would re-emit the row that slid up into the previous one.
  // A tiny batch keeps the sentinel on screen, so the batches append here.
  await page.goto("/?limit=2");
  await expect
    .poll(async () => page.locator("article.row").count(), { timeout: 15_000 })
    .toBeGreaterThan(2);
  const row = page.locator("article.row").first();
  const id = await rowID(row);
  expect(id).toBeTruthy();
  const before = Number(
    (await page.locator(".work-list__footer output").innerText()).replace(
      /\D+/g,
      "",
    ),
  );
  await row.getByRole("button", { name: "Mark up to date" }).click();
  await expect
    .poll(async () =>
      Number(
        (await page.locator(".work-list__footer output").innerText()).replace(
          /\D+/g,
          "",
        ),
      ),
    )
    .toBe(before - 1);
  await expect(
    page.locator("article.row").filter({ hasText: id ?? "" }),
  ).toHaveCount(0);
  // Every remaining row is still a distinct submission.
  const remaining = await page
    .locator("article.row .row__link")
    .evaluateAll((nodes) =>
      nodes.map((node) => (node as HTMLAnchorElement).getAttribute("href")),
    );
  expect(new Set(remaining).size).toBe(remaining.length);

  const activity = await request.get(
    `${process.env.SMOKE_PB_URL}/api/jserd/monitoring/submissions/${id}/activity`,
    { headers: { Authorization: await token(request) } },
  );
  const entries = (await activity.json()).filter(
    (item: { kind: string }) => item.kind === "journal",
  );
  expect(entries).toHaveLength(1);
  expect(entries[0].body).toBe("Set aside.");
});
