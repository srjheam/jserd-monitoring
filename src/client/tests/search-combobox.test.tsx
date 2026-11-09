import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { ClientProviders } from "../src/app/provider";
import { SearchCombobox } from "../src/components/search-combobox";
import type { EditorFrequency, SideScreen } from "../src/lib/monitoring";
import type { Query } from "../src/lib/query";

// Four names, matching the prototype's own fixture (branch
// prototype/search-combobox-keyboard-nav, commit 8c6e9c6): enough to
// exercise wrap-around at both ends of the suggestion list.
const fourEditors: EditorFrequency[] = [
  { slug: "alice-smith", label: "Alice Smith", count: 4 },
  { slug: "bob-jones", label: "Bob Jones", count: 3 },
  { slug: "carol-white", label: "Carol White", count: 2 },
  { slug: "dana-lee", label: "Dana Lee", count: 1 },
];

function renderCombo(
  options: {
    query?: Query;
    editorFrequencies?: EditorFrequency[];
    route?: SideScreen;
    onQuery?: (query: Query) => void;
    onNavigate?: (screen: SideScreen | undefined) => void;
  } = {},
) {
  return render(
    <ClientProviders>
      <SearchCombobox
        query={
          options.query ?? {
            chips: [{ tag: "folder", value: "work", label: "Work" }],
            text: "",
          }
        }
        editorFrequencies={options.editorFrequencies ?? []}
        route={options.route}
        onQuery={options.onQuery ?? (() => {})}
        onNavigate={options.onNavigate ?? (() => {})}
      />
    </ClientProviders>,
  );
}

describe("the search box's layout", () => {
  it("renders the box inside a full-width row, with the magnifier outside it", () => {
    const { container } = renderCombo();
    const row = container.querySelector(".search-row");
    const box = container.querySelector(".search");
    const magnifier = screen.getByRole("button", { name: "Run search" });
    expect(row).not.toBeNull();
    expect(box).not.toBeNull();
    expect(row?.contains(box)).toBe(true);
    expect(row?.contains(magnifier)).toBe(true);
    expect(box?.contains(magnifier)).toBe(false);
  });

  it("renders editor and folder tags as inline text tokens in entry order, not as leading chips", () => {
    const { container } = renderCombo({
      query: {
        chips: [
          { tag: "folder", value: "work", label: "Work" },
          {
            tag: "editor",
            value: "casey-lee",
            label: "Casey Lee",
          },
        ],
        text: "",
      },
    });
    expect(container.querySelector(".chip")).toBeNull();
    const tokens = container.querySelectorAll(".tag-token");
    expect(tokens).toHaveLength(2);
    expect(tokens[0].textContent).toContain("folder:Work");
    expect(tokens[1].textContent).toContain("editor:Casey Lee");
  });

  it("places the clear control inside the box's right inner edge", () => {
    const { container } = renderCombo();
    const box = container.querySelector(".search");
    const clear = screen.getByRole("button", { name: "Clear" });
    expect(box?.contains(clear)).toBe(true);
  });
});

describe("the search box's magnifier and Enter behaviour", () => {
  it("runs the same commit as Enter, and blurs the box", async () => {
    const user = userEvent.setup();
    const onQuery = vi.fn();
    renderCombo({ onQuery });
    const input = screen.getByRole("combobox") as HTMLInputElement;
    await user.type(input, "job ads");
    expect(onQuery).not.toHaveBeenCalled();
    await user.click(screen.getByRole("button", { name: "Run search" }));
    expect(onQuery).toHaveBeenCalledWith({
      chips: [{ tag: "folder", value: "work", label: "Work" }],
      text: "job ads",
    });
    expect(document.activeElement).not.toBe(input);
  });

  it("closes an open suggestion list on Enter, without blurring the box", async () => {
    const user = userEvent.setup();
    renderCombo({
      editorFrequencies: [{ slug: "casey-lee", label: "Casey Lee", count: 2 }],
    });
    const input = screen.getByRole("combobox") as HTMLInputElement;
    await user.click(input);
    expect(screen.getByRole("listbox")).toBeDefined();
    await user.keyboard("{Enter}");
    expect(screen.queryByRole("listbox")).toBeNull();
    expect(document.activeElement).toBe(input);
  });

  it("closes an open suggestion list when the magnifier is pressed", async () => {
    const user = userEvent.setup();
    renderCombo({
      editorFrequencies: [{ slug: "casey-lee", label: "Casey Lee", count: 2 }],
    });
    await user.click(screen.getByRole("combobox"));
    expect(screen.getByRole("listbox")).toBeDefined();
    await user.click(screen.getByRole("button", { name: "Run search" }));
    expect(screen.queryByRole("listbox")).toBeNull();
  });

  it("closes an open suggestion list on Enter mid chip-edit, even with nothing highlighted to commit", async () => {
    const user = userEvent.setup();
    const onQuery = vi.fn();
    renderCombo({
      query: {
        chips: [
          { tag: "folder", value: "work", label: "Work" },
          {
            tag: "editor",
            value: "casey-lee",
            label: "Casey Lee",
          },
        ],
        text: "",
      },
      onQuery,
    });
    const input = screen.getByRole("combobox") as HTMLInputElement;
    await user.click(input);
    // Down from the free-text field reaches the editor field, whose
    // suggestions open as soon as it has focus.
    await user.keyboard("{ArrowDown}");
    expect(screen.getByRole("listbox")).toBeDefined();
    await user.keyboard("{Enter}");
    expect(screen.queryByRole("listbox")).toBeNull();
    expect(onQuery).not.toHaveBeenCalled();
  });
});

// This model is the decision lifted from the throwaway prototype at branch
// prototype/search-combobox-keyboard-nav, commit 8c6e9c6 (spec.md).
describe("the search box's keyboard-navigation model", () => {
  it("happy path: Down to the editor field, Right opens its list, Down highlights the second name, Tab commits it and returns to free text", async () => {
    const user = userEvent.setup();
    const onQuery = vi.fn();
    renderCombo({ editorFrequencies: fourEditors, onQuery });
    const input = screen.getByRole("combobox") as HTMLInputElement;
    await user.click(input);
    await user.keyboard("{ArrowDown}");
    await user.keyboard("{ArrowRight}");
    await user.keyboard("{ArrowDown}");
    await user.keyboard("{Tab}");
    expect(onQuery).toHaveBeenCalledWith({
      chips: [
        { tag: "folder", value: "work", label: "Work" },
        { tag: "editor", value: "bob-jones", label: "Bob Jones" },
      ],
      text: "",
    });
    expect(document.activeElement).toBe(input);
  });

  it("commits the highlighted suggestion via keyboard even when it repeats the field's existing value", async () => {
    const user = userEvent.setup();
    const onQuery = vi.fn();
    renderCombo({ onQuery });
    const input = screen.getByRole("combobox") as HTMLInputElement;
    await user.click(input);
    // Up wraps straight to the folder field, whose first (and, here,
    // already-committed) suggestion is "Work".
    await user.keyboard("{ArrowUp}");
    await user.keyboard("{ArrowRight}");
    await user.keyboard("{ArrowRight}");
    expect(onQuery).toHaveBeenCalledWith({
      chips: [{ tag: "folder", value: "work", label: "Work" }],
      text: "",
    });
    expect(document.activeElement).toBe(input);
  });

  it("shows only the first four suggestions once a field has focus", async () => {
    const user = userEvent.setup();
    renderCombo({
      editorFrequencies: [
        ...fourEditors,
        { slug: "erin-park", label: "Erin Park", count: 1 },
      ],
    });
    const input = screen.getByRole("combobox");
    await user.click(input);
    await user.keyboard("{ArrowDown}");
    expect(screen.getAllByRole("option")).toHaveLength(4);
    expect(screen.queryByRole("option", { name: "Erin Park" })).toBeNull();
  });

  it("wraps Down around an interactive list, from the last suggestion back to the first, and never exits", async () => {
    const user = userEvent.setup();
    renderCombo({ editorFrequencies: fourEditors });
    const input = screen.getByRole("combobox");
    await user.click(input);
    await user.keyboard("{ArrowDown}{ArrowRight}");
    await user.keyboard("{ArrowDown}{ArrowDown}{ArrowDown}");
    await user.keyboard("{ArrowDown}");
    expect(
      screen
        .getByRole("option", { name: "Alice Smith" })
        .getAttribute("aria-selected"),
    ).toBe("true");
  });

  it("exits an interactive list only once Up is pressed past the first item", async () => {
    const user = userEvent.setup();
    renderCombo({ editorFrequencies: fourEditors });
    const input = screen.getByRole("combobox");
    await user.click(input);
    await user.keyboard("{ArrowDown}{ArrowRight}");
    await user.keyboard("{ArrowDown}{ArrowDown}");
    await user.keyboard("{ArrowUp}{ArrowUp}");
    expect(
      screen
        .getByRole("option", { name: "Alice Smith" })
        .getAttribute("aria-selected"),
    ).toBe("true");
    await user.keyboard("{ArrowUp}");
    const options = screen.getAllByRole("option");
    expect(
      options.every(
        (option) => option.getAttribute("aria-selected") === "false",
      ),
    ).toBe(true);
  });

  it("cycles the three fields in a full circle in both directions", async () => {
    const user = userEvent.setup();
    renderCombo({ editorFrequencies: fourEditors });
    const input = screen.getByRole("combobox");
    await user.click(input);
    // input -> folder, wrapping past the top boundary.
    await user.keyboard("{ArrowUp}");
    expect(screen.getByRole("option", { name: "Work" })).toBeDefined();
    // folder -> input, wrapping past the bottom boundary.
    await user.keyboard("{ArrowDown}");
    expect(screen.queryByText("Folder")).not.toBeNull();
    // input -> editor, a normal step.
    await user.keyboard("{ArrowDown}");
    expect(screen.getByRole("option", { name: "Alice Smith" })).toBeDefined();
    expect(screen.queryByText("Folder")).toBeNull();
  });

  it("exits an interactive list instantly on Left arrow, from any highlighted position", async () => {
    const user = userEvent.setup();
    renderCombo({ editorFrequencies: fourEditors });
    const input = screen.getByRole("combobox");
    await user.click(input);
    await user.keyboard("{ArrowDown}{ArrowRight}{ArrowDown}");
    await user.keyboard("{ArrowLeft}");
    const options = screen.getAllByRole("option");
    expect(
      options.every(
        (option) => option.getAttribute("aria-selected") === "false",
      ),
    ).toBe(true);
  });

  it("exits an interactive list instantly on Shift+Tab, from any highlighted position", async () => {
    const user = userEvent.setup();
    renderCombo({ editorFrequencies: fourEditors });
    const input = screen.getByRole("combobox") as HTMLInputElement;
    await user.click(input);
    await user.keyboard("{ArrowDown}{ArrowRight}{ArrowDown}");
    await user.keyboard("{Shift>}{Tab}{/Shift}");
    const options = screen.getAllByRole("option");
    expect(
      options.every(
        (option) => option.getAttribute("aria-selected") === "false",
      ),
    ).toBe(true);
    expect(document.activeElement).toBe(input);
  });

  it("does not intercept Tab on the free-text field, handing off to the page's own tab order", async () => {
    const user = userEvent.setup();
    const onQuery = vi.fn();
    renderCombo({ onQuery });
    const input = screen.getByRole("combobox") as HTMLInputElement;
    await user.click(input);
    await user.keyboard("{Tab}");
    expect(document.activeElement).not.toBe(input);
    expect(onQuery).not.toHaveBeenCalled();
  });
});
