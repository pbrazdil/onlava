import { cn } from "@/lib/utils";

export function sidebarItemClass(active: boolean) {
  return cn(
    "relative flex h-7 items-center justify-between gap-2 rounded-md px-2.5 text-[13px] font-medium leading-5 transition-colors duration-150 ease-out focus-visible:ring-2 focus-visible:ring-app-focus-soft focus-visible:outline-none",
    active
      ? "bg-app-sidebar-active text-app-sidebar-active-text shadow-[var(--app-sidebar-active-shadow)] before:absolute before:left-0 before:top-1/2 before:h-5 before:w-[2px] before:-translate-y-1/2 before:rounded-full before:bg-app-selection-ring"
      : "text-app-sidebar-muted hover:bg-app-sidebar-hover hover:text-app-sidebar-active-text",
  );
}

export function SidebarItemContent({ label, count }: { label: string; count: number }) {
  return (
    <>
      <span className="min-w-0 truncate">{label}</span>
      <span className="text-[12px] tabular-nums text-muted-foreground">{count}</span>
    </>
  );
}
