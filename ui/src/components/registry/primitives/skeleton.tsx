import { cn } from "@/lib/utils"

function Skeleton({ className, ...props }: React.ComponentProps<"div">) {
  return (
    <div
      data-slot="skeleton"
      className={cn("rounded-2xl bg-muted opacity-60", className)}
      {...props}
    />
  )
}

export { Skeleton }
