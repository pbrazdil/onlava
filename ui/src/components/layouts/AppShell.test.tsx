import { render, screen } from "@testing-library/react";
import {
  AppShell,
  appShellAppMenuButtonClass,
  appShellNavItemClass,
} from "./AppShell";

describe("AppShell", () => {
  it("renders stable shell slots and compile errors", () => {
    const { container } = render(
      <AppShell
        topbar={<nav aria-label="Test nav">Navigation</nav>}
        compileError={<div>compile failed</div>}
      >
        <main>Dashboard body</main>
      </AppShell>,
    );

    expect(container.querySelector('[data-onlava-ui="AppShell"]')).not.toBeNull();
    expect(container.querySelector('[data-slot="topbar"]')).not.toBeNull();
    expect(container.querySelector('[data-slot="body"]')).not.toBeNull();
    expect(container.querySelector('[data-slot="compile-error"]')).not.toBeNull();
    expect(screen.getByText("Navigation")).toBeTruthy();
    expect(screen.getByText("Dashboard body")).toBeTruthy();
    expect(screen.getByText("compile failed")).toBeTruthy();
  });

  it("keeps app navigation styling in the layout layer", () => {
    expect(appShellNavItemClass(true)).toContain("bg-sidebar-accent");
    expect(appShellNavItemClass(false, true)).toContain("opacity-90");
    expect(appShellAppMenuButtonClass()).toContain("cursor-pointer");
  });
});
