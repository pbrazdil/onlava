import { useTheme } from "next-themes";
import {
	Loader2Icon,
	CircleCheckIcon,
	InfoIcon,
	OctagonXIcon,
	TriangleAlertIcon,
} from "lucide-react";
import {
	Toaster as BaseSonner,
	type ToasterProps as SonnerProps,
} from "sonner";

const Sonner = ({ ...props }: SonnerProps) => {
	const { theme = "system" } = useTheme();

	return (
		<BaseSonner
			theme={theme as SonnerProps["theme"]}
			className="toaster group"
			icons={{
				success: <CircleCheckIcon className="size-4" />,
				info: <InfoIcon className="size-4" />,
				warning: <TriangleAlertIcon className="size-4" />,
				error: <OctagonXIcon className="size-4" />,
				loading: <Loader2Icon className="size-4 animate-spin" />,
			}}
			style={
				{
					"--normal-bg": "var(--popover)",
					"--normal-text": "var(--popover-foreground)",
					"--normal-border": "var(--border)",
					"--border-radius": "var(--radius)",
				} as React.CSSProperties
			}
			toastOptions={{
				classNames: {
					toast: "cn-toast",
					title: "text-[#202223]",
					description: "text-[#6b665c] opacity-100",
				},
			}}
			{...props}
		/>
	);
};

export { Sonner };
