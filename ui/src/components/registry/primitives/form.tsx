"use client";

import type * as LabelPrimitive from "@radix-ui/react-label";
import { Slot } from "@radix-ui/react-slot";
import {
	createFormHook,
	createFormHookContexts,
	type ValidationError,
} from "@tanstack/react-form";
import * as React from "react";
import { Label } from "@/components/primitives/label";
import { cn } from "@/lib/utils";

const { fieldContext, useFieldContext, formContext, useFormContext } =
	createFormHookContexts();

const { useAppForm, withForm } = createFormHook({
	fieldComponents: {},
	formComponents: {},
	fieldContext,
	formContext,
});

type FormItemContextValue = {
	id: string;
};

const FormItemContext = React.createContext<FormItemContextValue | undefined>(
	undefined,
);

function getErrorMessages(error: ValidationError): string[] {
	if (error == null) {
		return [];
	}

	if (typeof error === "string") {
		return [error];
	}

	if (error instanceof Error) {
		return error.message ? [error.message] : [];
	}

	if (Array.isArray(error)) {
		return error.flatMap((entry) => getErrorMessages(entry));
	}

	if (typeof error === "object" && "message" in error) {
		const message = error.message;
		return typeof message === "string" && message.length > 0 ? [message] : [];
	}

	return [];
}

const useFormField = () => {
	const field = useFieldContext<unknown>();
	const itemContext = React.useContext(FormItemContext);

	if (!itemContext) {
		throw new Error("useFormField should be used within <FormItem>");
	}

	const errors = field.state.meta.errors.flatMap((error) => getErrorMessages(error));
	const uniqueErrors = [...new Set(errors)];
	const hasErrors = uniqueErrors.length > 0;
	const { id } = itemContext;

	return {
		error: uniqueErrors[0],
		errors: uniqueErrors,
		formItemId: `${id}-form-item`,
		formDescriptionId: `${id}-form-item-description`,
		formMessageId: `${id}-form-item-message`,
		hasErrors,
		name: field.name,
	};
};

function FormItem({ className, ...props }: React.ComponentProps<"div">) {
	const id = React.useId();

	return (
		<FormItemContext.Provider value={{ id }}>
			<div
				data-slot="form-item"
				className={cn("grid gap-2", className)}
				{...props}
			/>
		</FormItemContext.Provider>
	);
}

function FormLabel({
	className,
	...props
}: React.ComponentProps<typeof LabelPrimitive.Root>) {
	const { hasErrors, formItemId } = useFormField();

	return (
		<Label
			data-slot="form-label"
			data-error={hasErrors}
			className={cn("data-[error=true]:text-destructive", className)}
			htmlFor={formItemId}
			{...props}
		/>
	);
}

function FormControl({ ...props }: React.ComponentProps<typeof Slot>) {
	const { hasErrors, formItemId, formDescriptionId, formMessageId } =
		useFormField();

	return (
		<Slot
			data-slot="form-control"
			id={formItemId}
			aria-describedby={
				hasErrors
					? `${formDescriptionId} ${formMessageId}`
					: formDescriptionId
			}
			aria-invalid={hasErrors}
			{...props}
		/>
	);
}

function FormDescription({ className, ...props }: React.ComponentProps<"p">) {
	const { formDescriptionId } = useFormField();

	return (
		<p
			data-slot="form-description"
			id={formDescriptionId}
			className={cn("text-muted-foreground text-sm", className)}
			{...props}
		/>
	);
}

function FormMessage({ className, ...props }: React.ComponentProps<"div">) {
	const { errors, formMessageId } = useFormField();
	const body =
		errors.length > 1 ? (
			<ul className="ml-4 flex list-disc flex-col gap-1">
				{errors.map((error) => (
					<li key={error}>{error}</li>
				))}
			</ul>
		) : errors[0] ?? props.children;

	if (!body) {
		return null;
	}

	return (
		<div
			data-slot="form-message"
			id={formMessageId}
			className={cn("text-destructive text-sm", className)}
			{...props}
		>
			{body}
		</div>
	);
}

export {
	FormControl,
	FormDescription,
	FormItem,
	FormLabel,
	FormMessage,
	useAppForm,
	useFieldContext as useAppFieldContext,
	useFormContext as useAppFormContext,
	useFormField,
	withForm,
};
