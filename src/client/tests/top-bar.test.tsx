import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, expect, it, vi } from "vitest";
import { ClientProviders } from "../src/app/provider";
import { TopBar } from "../src/components/top-bar";

const renderTopBar = (
  onSignOut: () => void = () => {},
  onManageConferences: () => void = () => {},
) =>
  render(
    <ClientProviders>
      <TopBar
        meta={null}
        user="editor@example.test"
        onSignOut={onSignOut}
        onManageConferences={onManageConferences}
      />
    </ClientProviders>,
  );

// Real matchMedia is stubbed desktop-only in tests/setup.ts; the mobile
// tests below narrow the query themselves and restore it afterward.
let restoreMatchMedia: (() => void) | undefined;

const setCompact = (matches: boolean) => {
  const original = window.matchMedia;
  window.matchMedia = (query: string) =>
    ({
      matches: query === "(max-width: 767px)" ? matches : false,
      media: query,
      onchange: null,
      addEventListener: () => {},
      removeEventListener: () => {},
      addListener: () => {},
      removeListener: () => {},
      dispatchEvent: () => false,
    }) as unknown as MediaQueryList;
  restoreMatchMedia = () => {
    window.matchMedia = original;
  };
};

afterEach(() => {
  restoreMatchMedia?.();
  restoreMatchMedia = undefined;
});

// An open popover hides the rest of the page from queries, so every step after
// a menu interaction uses findBy* and waits for the popover to finish closing.
it("renders in English and switches locale without reloading", async () => {
  const user = userEvent.setup();
  const before = window.location.href;
  renderTopBar();

  const banner = screen.getByRole("banner");
  expect(
    screen.getByRole("heading", { name: "Editorial monitoring" }),
  ).toBeDefined();
  const language = await screen.findByRole("button", { name: /Language/ });
  expect(language.textContent).toContain("English");

  await user.click(language);
  await user.click(
    await screen.findByRole("menuitemradio", { name: "Português" }),
  );

  expect(
    await screen.findByRole("heading", { name: "Monitoramento editorial" }),
  ).toBeDefined();
  expect(
    (await screen.findByRole("button", { name: /Idioma/ })).textContent,
  ).toContain("Português");
  expect(document.documentElement.lang).toBe("pt-BR");
  // Same DOM node and same URL: the switch re-rendered, it did not navigate.
  expect(screen.getByRole("banner")).toBe(banner);
  expect(window.location.href).toBe(before);
});

it("signs out from the user menu", async () => {
  const user = userEvent.setup();
  const onSignOut = vi.fn();
  renderTopBar(onSignOut);
  await user.click(await screen.findByRole("button", { name: /Account/ }));
  await user.click(await screen.findByRole("menuitem", { name: "Sign out" }));
  expect(onSignOut).toHaveBeenCalledOnce();
});

it("opens Manage conferences from the hamburger at desktop width", async () => {
  const user = userEvent.setup();
  const onManageConferences = vi.fn();
  renderTopBar(() => {}, onManageConferences);
  await user.click(await screen.findByRole("button", { name: "Menu" }));
  await user.click(
    await screen.findByRole("menuitem", { name: "Manage conferences" }),
  );
  expect(onManageConferences).toHaveBeenCalledOnce();
  expect(screen.getByRole("button", { name: /Language/ })).toBeDefined();
  expect(screen.getByRole("button", { name: /Account/ })).toBeDefined();
});

it("moves Language and Account into the hamburger at mobile width", async () => {
  setCompact(true);
  const user = userEvent.setup();
  const onManageConferences = vi.fn();
  renderTopBar(() => {}, onManageConferences);

  expect(screen.queryByRole("button", { name: /Language/ })).toBeNull();
  expect(screen.queryByRole("button", { name: /Account/ })).toBeNull();

  await user.click(await screen.findByRole("button", { name: "Menu" }));
  await user.click(
    await screen.findByRole("menuitem", { name: "Manage conferences" }),
  );
  expect(onManageConferences).toHaveBeenCalledOnce();

  await user.click(await screen.findByRole("button", { name: "Menu" }));
  expect(
    await screen.findByRole("menuitem", { name: "Português" }),
  ).toBeDefined();
  expect(
    await screen.findByRole("menuitem", { name: "Sign out" }),
  ).toBeDefined();
});
