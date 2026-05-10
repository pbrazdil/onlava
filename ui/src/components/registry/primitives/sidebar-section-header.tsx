import { ChevronRight, Plus } from "lucide-react";
import type * as React from "react";

import { cn } from "@/lib/utils";

type SidebarSectionHeaderProps = {
	icon: React.ComponentType<{
		className?: string;
		strokeWidth?: number;
	}>;
	title: string;
	addLabel: string;
	optionsLabel: string;
	onAdd?: () => void;
	onOptions?: () => void;
};

export function SidebarSectionHeader({
	icon: Icon,
	title,
	addLabel,
	optionsLabel,
	onAdd,
	onOptions,
}: SidebarSectionHeaderProps) {
	return (
		<div className="flex items-center justify-between gap-2 px-2 py-1.5">
			<div className="flex min-w-0 items-center gap-2 text-[12px] font-semibold text-app-sidebar-active-text">
				<Icon className="size-4 shrink-0 text-app-icon-muted" />
				{title}
			</div>
			<div className="flex items-center gap-1">
				<button
					type="button"
					className={cn(
						"flex size-7 items-center justify-center rounded-md text-app-sidebar-muted transition-colors",
						"hover:bg-app-sidebar-hover hover:text-app-sidebar-active-text",
						"focus-visible:ring-2 focus-visible:ring-app-focus-soft focus-visible:outline-none",
					)}
					aria-label={addLabel}
					onClick={onAdd}
				>
					<Plus className="size-4" strokeWidth={2.5} />
				</button>
				<button
					type="button"
					className="flex size-7 items-center justify-center rounded-md text-app-sidebar-muted transition-colors hover:bg-app-sidebar-hover hover:text-app-sidebar-active-text focus-visible:ring-2 focus-visible:ring-app-focus-soft focus-visible:outline-none"
					aria-label={optionsLabel}
					onClick={onOptions}
				>
					<ChevronRight className="size-4" strokeWidth={2.5} />
				</button>
			</div>
		</div>
	);
}
