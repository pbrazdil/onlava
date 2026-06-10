import type { ComponentProps, HTMLAttributes, ReactNode } from "react";
import { cn } from "@/lib/utils";
import { SelectTrigger } from "@/components/primitives/select";

export function AppSidebar({
	fill,
	className,
	...props
}: HTMLAttributes<HTMLElement> & { fill?: boolean }) {
	return (
		<aside
			data-scenery-ui="AppSidebar"
			className={cn(
				"w-[230px] shrink-0 overflow-hidden rounded-lg border border-app-separator-subtle bg-app-sidebar-surface",
				fill && "flex min-h-0 w-full flex-col",
				className,
			)}
			{...props}
		/>
	);
}

export function AppTwoPane({
	className,
	...props
}: HTMLAttributes<HTMLDivElement>) {
	return (
		<div
			data-scenery-ui="AppTwoPane"
			className={cn(
				"grid min-h-0 flex-1 gap-2 p-2 xl:grid-cols-[320px_minmax(0,1fr)]",
				className,
			)}
			{...props}
		/>
	);
}

export function AppMain({
	className,
	...props
}: HTMLAttributes<HTMLElement>) {
	return (
		<main
			data-scenery-ui="AppMain"
			className={cn(
				"flex min-w-0 flex-1 flex-col overflow-hidden rounded-lg bg-app-work-surface",
				className,
			)}
			{...props}
		/>
	);
}

export function AppHeader({
	className,
	...props
}: HTMLAttributes<HTMLElement>) {
	return (
		<header
			data-scenery-ui="AppHeader"
			className={cn(
				"flex min-h-14 shrink-0 items-center justify-between gap-3 border-b border-app-separator-subtle px-[18px]",
				className,
			)}
			{...props}
		/>
	);
}

export function AppToolbar({
	className,
	...props
}: HTMLAttributes<HTMLDivElement>) {
	return (
		<div
			data-scenery-ui="AppToolbar"
			className={cn(
				"border-b border-app-separator-subtle bg-app-toolbar-surface p-3",
				className,
			)}
			{...props}
		/>
	);
}

export function AppFilterControl({
	label,
	children,
	className,
}: {
	label: ReactNode;
	children: ReactNode;
	className?: string;
}) {
	return (
		<div
			data-scenery-ui="AppFilterControl"
			className={cn(
				"flex min-w-0 items-center gap-1 rounded-full border border-border/80 bg-background px-2 py-1",
				className,
			)}
		>
			<span className="text-[10px] font-medium uppercase tracking-wide text-muted-foreground">
				{label}
			</span>
			{children}
		</div>
	);
}

export function AppFilterSelectTrigger({
	className,
	size = "sm",
	...props
}: ComponentProps<typeof SelectTrigger>) {
	return (
		<SelectTrigger
			data-scenery-ui="AppFilterSelectTrigger"
			size={size}
			className={cn(
				"h-6 min-w-0 flex-1 border-0 bg-transparent px-1 py-0 text-xs shadow-none focus-visible:ring-0",
				className,
			)}
			{...props}
		/>
	);
}

export function AppPanel({
	className,
	...props
}: HTMLAttributes<HTMLElement>) {
	return (
		<section
			data-scenery-ui="AppPanel"
			className={cn(
				"rounded-lg border border-app-separator-subtle bg-app-panel-surface",
				className,
			)}
			{...props}
		/>
	);
}

export function AppMetaBox({
	className,
	...props
}: HTMLAttributes<HTMLDivElement>) {
	return (
		<div
			data-scenery-ui="AppMetaBox"
			className={cn(
				"rounded-md border border-app-separator-subtle bg-app-field-surface px-2 py-2 text-[12px]",
				className,
			)}
			{...props}
		/>
	);
}
