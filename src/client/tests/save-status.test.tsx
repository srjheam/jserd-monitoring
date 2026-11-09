import { render, screen } from "@testing-library/react";
import { expect, it } from "vitest";
import { SaveStatus } from "../src/components/save-status";

it("renders the lucide success icon when a save succeeds", () => {
  render(
    <SaveStatus status="success" savingLabel="Saving…" savedLabel="Saved" />,
  );
  const output = screen.getByText("Saved").closest("output");
  expect(output?.querySelector(".lucide-check")).toBeDefined();
});
