import AxeBuilder from "@axe-core/playwright";
import { expect, test } from "@playwright/test";

const email = process.env.SMOKE_EMAIL || "operator@example.test";
const password = process.env.SMOKE_PASSWORD || "valid-password";
const pocketBaseURL = process.env.SMOKE_PB_URL || "http://127.0.0.1:18097";

test("the submission page carries activity, curation and a keyboard tooltip", async ({
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

  await page.goto("/");
  await page.getByLabel("Email").fill(email);
  await page.getByLabel("Password").fill(password);
  await page.getByRole("button", { name: "Sign in" }).click();
  await expect(
    page.getByRole("heading", { name: "Editorial monitoring" }),
  ).toBeVisible();

  // `note=1` is the signal the work list's note control sends: open this
  // submission with the note box already focused.
  await page.goto(`/?article=${submissionID}&note=1`);
  const noteBox = page.getByRole("textbox", { name: "Write a note…" });
  await expect(noteBox).toBeFocused();

  // The entry is appended by the keyboard alone, and comes back from the
  // server as timeline text with no control to change it.
  const body = `Smoke note ${Date.now()}`;
  await noteBox.fill(body);
  await noteBox.press("Control+Enter");
  await expect(page.getByText(body)).toBeVisible();
  await expect(noteBox).toHaveValue("");
  // The saved entry is plain text: the note box is still the only writing
  // surface, and nothing offers to change the entry.
  await expect(
    page.getByRole("textbox", { name: "Write a note…" }),
  ).toHaveCount(1);
  await expect(page.getByRole("textbox", { name: body })).toHaveCount(0);
  expect(
    await page
      .getByText(body)
      .evaluate((node) => (node as HTMLElement).isContentEditable),
  ).toBe(false);

  await expect(page.getByText("Field provenance")).toHaveCount(0);
  await expect(
    page.getByRole("button", { name: /local tracking/i }),
  ).toHaveCount(0);

  const axe = await new AxeBuilder({ page }).analyze();
  expect(axe.violations).toEqual([]);

  // The ownership tooltip must answer a keyboard, not only a mouse. Tabbing
  // to the info icon is what a keyboard reader does, and it is also what
  // puts the interface into keyboard focus mode. The tooltip is portalled
  // outside every landmark while it is open, so this runs after the axe
  // pass.
  // The label and the ⓘ icon are two separate triggers for the same
  // tooltip; the icon is the second one in DOM order.
  const info = page
    .getByRole("button", { name: "About the conference field" })
    .last();
  // Tab to it, then press it: that is the keyboard route to a button, and it
  // is deterministic. Calling focus() alone is programmatic focus, which does
  // not put react-aria into keyboard modality, so the tooltip stayed shut.
  await info.press("Enter");
  await expect(
    page.getByText(
      "Conference associations are managed in JSERD Monitoring separately from OJS.",
    ),
  ).toBeVisible();
});
