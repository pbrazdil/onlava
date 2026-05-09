import { ChevronDown } from "lucide-react";
import type * as React from "react";

type FilterPillProps = {
	icon?: React.ComponentType<{
		className?: string;
		strokeWidth?: number;
	}>;
	label: string;
};

export function FilterPill({ icon: Icon, label }: FilterPillProps) {
	return (
		<div className="inline-flex h-8 items-center gap-2 rounded-md border border-[var(--pulse-separator-subtle)] bg-[var(--pulse-field-surface)] px-3 text-[13px]">
			{Icon ? <Icon className="size-4 text-[var(--pulse-icon-muted)]" /> : null}
			<span className="text-foreground">{label}</span>
			<ChevronDown className="size-4 text-[var(--pulse-icon-muted)]" />
		</div>
	);
}
