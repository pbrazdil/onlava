import { render, screen } from "@testing-library/react";
import {
  ProductHeader,
  ProductMain,
  ProductMetaBox,
  ProductPanel,
  ProductSidebar,
  ProductToolbar,
} from "./ProductSurface";

describe("ProductSurface", () => {
  it("renders stable product layout markers with ONLV-compatible chrome classes", () => {
    const { container } = render(
      <div>
        <ProductSidebar>Sidebar</ProductSidebar>
        <ProductMain>
          <ProductHeader>Header</ProductHeader>
          <ProductToolbar>Toolbar</ProductToolbar>
          <ProductPanel>
            <ProductMetaBox>Meta</ProductMetaBox>
          </ProductPanel>
        </ProductMain>
      </div>,
    );

    const sidebar = container.querySelector('[data-onlava-ui="ProductSidebar"]');
    const main = container.querySelector('[data-onlava-ui="ProductMain"]');
    const header = container.querySelector('[data-onlava-ui="ProductHeader"]');
    const toolbar = container.querySelector('[data-onlava-ui="ProductToolbar"]');
    const panel = container.querySelector('[data-onlava-ui="ProductPanel"]');
    const meta = container.querySelector('[data-onlava-ui="ProductMetaBox"]');

    expect(sidebar?.tagName).toBe("ASIDE");
    expect(main?.tagName).toBe("MAIN");
    expect(header?.tagName).toBe("HEADER");
    expect(toolbar?.tagName).toBe("DIV");
    expect(panel?.tagName).toBe("SECTION");
    expect(meta?.tagName).toBe("DIV");

    expect(sidebar?.getAttribute("class")).toContain("w-[230px]");
    expect(main?.getAttribute("class")).toContain("bg-[var(--pulse-work-surface)]");
    expect(header?.getAttribute("class")).toContain("min-h-14");
    expect(toolbar?.getAttribute("class")).toContain("bg-[var(--pulse-toolbar-surface)]");
    expect(panel?.getAttribute("class")).toContain("bg-[var(--pulse-panel-surface)]");
    expect(meta?.getAttribute("class")).toContain("bg-[var(--pulse-field-surface)]");

    expect(screen.getByText("Sidebar")).toBeTruthy();
    expect(screen.getByText("Header")).toBeTruthy();
    expect(screen.getByText("Toolbar")).toBeTruthy();
    expect(screen.getByText("Meta")).toBeTruthy();
  });
});
