import type { ReactNode } from "react";
import { Button, type ButtonProps } from "@/components/primitives/Button";
import { cn } from "@/lib/utils";

export type PageToolbarAction = {
  label: string;
  key?: string;
  onClick?: () => void;
  disabled?: boolean;
  tone?: ButtonProps["tone"];
  leadingIcon?: ReactNode;
};

export type PageToolbarProps = {
  primaryAction?: PageToolbarAction;
  secondaryActions?: PageToolbarAction[];
  children?: ReactNode;
  className?: string;
};

export function PageToolbar({ primaryAction, secondaryActions = [], children, className }: PageToolbarProps) {
  return (
    <div data-onlava-ui="PageToolbar" className={cn("flex items-center justify-end gap-2", className)}>
      {children}
      {secondaryActions.map((action, index) => (
        <Button
          key={action.key ?? `${action.label}-${index}`}
          size="sm"
          tone={action.tone ?? "secondary"}
          leadingIcon={action.leadingIcon}
          disabled={action.disabled}
          onClick={action.onClick}
        >
          {action.label}
        </Button>
      ))}
      {primaryAction ? (
        <Button
          size="sm"
          tone={primaryAction.tone ?? "primary"}
          leadingIcon={primaryAction.leadingIcon}
          disabled={primaryAction.disabled}
          onClick={primaryAction.onClick}
        >
          {primaryAction.label}
        </Button>
      ) : null}
    </div>
  );
}
