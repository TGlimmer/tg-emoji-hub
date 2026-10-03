import {
  createContext,
  forwardRef,
  useContext,
  useId,
  type ButtonHTMLAttributes,
  type HTMLAttributes,
} from "react";
import { cn } from "@/lib/utils";

// 轻量 Tabs：受控 / 非受控两用，无需 @radix-ui，匹配 skills.sh 极简风
interface TabsContextValue {
  value: string;
  setValue: (next: string) => void;
  baseId: string;
}

const TabsContext = createContext<TabsContextValue | null>(null);

function useTabs() {
  const ctx = useContext(TabsContext);
  if (!ctx) throw new Error("Tabs.* 必须在 <Tabs> 内使用");
  return ctx;
}

interface TabsProps extends Omit<HTMLAttributes<HTMLDivElement>, "onChange"> {
  value: string;
  onValueChange: (next: string) => void;
}

export function Tabs({
  value,
  onValueChange,
  className,
  children,
  ...rest
}: TabsProps) {
  const baseId = useId();
  return (
    <TabsContext.Provider value={{ value, setValue: onValueChange, baseId }}>
      <div className={cn("flex flex-col gap-3", className)} {...rest}>
        {children}
      </div>
    </TabsContext.Provider>
  );
}

export const TabsList = forwardRef<HTMLDivElement, HTMLAttributes<HTMLDivElement>>(
  ({ className, ...rest }, ref) => (
    <div
      ref={ref}
      role="tablist"
      className={cn(
        "inline-flex h-9 items-center gap-1 rounded-md border border-border/60 bg-muted/40 p-1",
        className,
      )}
      {...rest}
    />
  ),
);
TabsList.displayName = "TabsList";

interface TabsTriggerProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  value: string;
}

export const TabsTrigger = forwardRef<HTMLButtonElement, TabsTriggerProps>(
  ({ value, className, children, ...rest }, ref) => {
    const { value: active, setValue, baseId } = useTabs();
    const selected = active === value;
    return (
      <button
        ref={ref}
        type="button"
        role="tab"
        id={`${baseId}-trigger-${value}`}
        aria-selected={selected}
        aria-controls={`${baseId}-panel-${value}`}
        tabIndex={selected ? 0 : -1}
        onClick={() => setValue(value)}
        className={cn(
          "inline-flex h-7 items-center justify-center rounded-sm px-3 text-xs font-medium transition-colors focus-visible:outline-hidden focus-visible:ring-2 focus-visible:ring-ring",
          selected
            ? "bg-background text-foreground shadow-sm"
            : "text-muted-foreground hover:text-foreground",
          className,
        )}
        {...rest}
      >
        {children}
      </button>
    );
  },
);
TabsTrigger.displayName = "TabsTrigger";

interface TabsContentProps extends HTMLAttributes<HTMLDivElement> {
  value: string;
}

export const TabsContent = forwardRef<HTMLDivElement, TabsContentProps>(
  ({ value, className, ...rest }, ref) => {
    const { value: active, baseId } = useTabs();
    if (active !== value) return null;
    return (
      <div
        ref={ref}
        role="tabpanel"
        id={`${baseId}-panel-${value}`}
        aria-labelledby={`${baseId}-trigger-${value}`}
        className={cn("focus-visible:outline-hidden", className)}
        {...rest}
      />
    );
  },
);
TabsContent.displayName = "TabsContent";
