import { render, screen } from "@testing-library/react";
import {
  AppFilterControl,
  AppHeader,
  AppMain,
  AppMetaBox,
  AppPanel,
  AppSidebar,
  AppToolbar,
  AppTwoPane,
} from "./AppSurface";

describe("AppSurface", () => {
  it("renders stable app layout markers with scenery app chrome classes", () => {
    const { container } = render(
      <div>
        <AppSidebar>Sidebar</AppSidebar>
        <AppMain>
          <AppHeader>Header</AppHeader>
          <AppToolbar>Toolbar</AppToolbar>
          <AppPanel>
            <AppMetaBox>Meta</AppMetaBox>
          </AppPanel>
        </AppMain>
      </div>,
    );

    const sidebar = container.querySelector('[data-scenery-ui="AppSidebar"]');
    const main = container.querySelector('[data-scenery-ui="AppMain"]');
    const header = container.querySelector('[data-scenery-ui="AppHeader"]');
    const toolbar = container.querySelector('[data-scenery-ui="AppToolbar"]');
    const panel = container.querySelector('[data-scenery-ui="AppPanel"]');
    const meta = container.querySelector('[data-scenery-ui="AppMetaBox"]');

    expect(sidebar?.tagName).toBe("ASIDE");
    expect(main?.tagName).toBe("MAIN");
    expect(header?.tagName).toBe("HEADER");
    expect(toolbar?.tagName).toBe("DIV");
    expect(panel?.tagName).toBe("SECTION");
    expect(meta?.tagName).toBe("DIV");

    expect(sidebar?.getAttribute("class")).toContain("w-[230px]");
    expect(main?.getAttribute("class")).toContain("bg-app-work-surface");
    expect(header?.getAttribute("class")).toContain("min-h-14");
    expect(toolbar?.getAttribute("class")).toContain("bg-app-toolbar-surface");
    expect(panel?.getAttribute("class")).toContain("bg-app-panel-surface");
    expect(meta?.getAttribute("class")).toContain("bg-app-field-surface");

    expect(screen.getByText("Sidebar")).toBeTruthy();
    expect(screen.getByText("Header")).toBeTruthy();
    expect(screen.getByText("Toolbar")).toBeTruthy();
    expect(screen.getByText("Meta")).toBeTruthy();
  });

  it("renders app composition helpers", () => {
    const { container } = render(
      <AppTwoPane>
        <AppFilterControl label="Stage">Active</AppFilterControl>
      </AppTwoPane>,
    );

    expect(container.querySelector('[data-scenery-ui="AppTwoPane"]')).toBeTruthy();
    expect(container.querySelector('[data-scenery-ui="AppFilterControl"]')).toBeTruthy();
    expect(screen.getByText("Stage")).toBeTruthy();
    expect(screen.getByText("Active")).toBeTruthy();
  });
});
