import {
  useEffect,
  useId,
  useRef,
  type ButtonHTMLAttributes,
  type InputHTMLAttributes,
  type ReactNode,
} from "react";

type Variant = "primary" | "secondary" | "danger" | "ghost";

const variantClasses: Record<Variant, string> = {
  primary: "bg-blue-600 text-white hover:bg-blue-700 disabled:bg-blue-300",
  secondary:
    "bg-white text-gray-800 border border-gray-300 hover:bg-gray-50 disabled:opacity-50",
  danger: "bg-red-600 text-white hover:bg-red-700 disabled:bg-red-300",
  ghost: "bg-transparent text-gray-600 hover:bg-gray-100 disabled:opacity-50",
};

export function Button({
  variant = "primary",
  className = "",
  ...props
}: Readonly<ButtonHTMLAttributes<HTMLButtonElement> & { variant?: Variant }>) {
  return (
    <button
      className={`inline-flex items-center justify-center gap-2 rounded-lg px-4 py-2 text-sm font-medium transition disabled:cursor-not-allowed ${variantClasses[variant]} ${className}`}
      {...props}
    />
  );
}

export function Input({
  className = "",
  ...props
}: Readonly<InputHTMLAttributes<HTMLInputElement>>) {
  return (
    <input
      className={`w-full rounded-lg border border-gray-300 px-3 py-2 text-sm outline-none focus:border-blue-500 focus:ring-2 focus:ring-blue-100 ${className}`}
      {...props}
    />
  );
}

export function Field({
  label,
  children,
}: Readonly<{
  label: string;
  children: ReactNode;
}>) {
  return (
    <label className="block space-y-1.5">
      <span className="text-sm font-medium text-gray-700">{label}</span>
      {children}
    </label>
  );
}

export function Card({
  children,
  className = "",
}: Readonly<{
  children: ReactNode;
  className?: string;
}>) {
  return (
    <div
      className={`rounded-xl border border-gray-200 bg-white shadow-sm ${className}`}
    >
      {children}
    </div>
  );
}

export function Spinner({ className = "" }: Readonly<{ className?: string }>) {
  return (
    <span
      className={`inline-block h-5 w-5 animate-spin rounded-full border-2 border-gray-300 border-t-blue-600 ${className}`}
    />
  );
}

export function Modal({
  open,
  onClose,
  title,
  children,
}: Readonly<{
  open: boolean;
  onClose?: () => void;
  title: string;
  children: ReactNode;
}>) {
  const titleId = useId();
  const ref = useRef<HTMLDialogElement>(null);

  // Open and close the native modal in step with `open`. showModal() gives the top-layer
  // render, focus trap and inert background for free; the guard keeps StrictMode's
  // double-invoked effect from calling showModal() on an already-open dialog.
  useEffect(() => {
    const dialog = ref.current;
    if (!dialog) return;
    if (!dialog.open) dialog.showModal();
    return () => {
      if (dialog.open) dialog.close();
    };
  }, [open]);

  // Dismissal: Escape (the dialog's cancel event) and clicks on the ::backdrop, which surface
  // with the dialog element itself as the target. These are native listeners rather than JSX
  // props so the non-interactive <dialog> carries no click/key handlers. cancel is always
  // prevented, so a missing onClose makes the dialog non-dismissible to match such callers.
  useEffect(() => {
    const dialog = ref.current;
    if (!dialog) return;
    const onCancel = (e: Event) => {
      e.preventDefault();
      onClose?.();
    };
    const onBackdropClick = (e: MouseEvent) => {
      if (onClose && e.target === dialog) onClose();
    };
    dialog.addEventListener("cancel", onCancel);
    dialog.addEventListener("click", onBackdropClick);
    return () => {
      dialog.removeEventListener("cancel", onCancel);
      dialog.removeEventListener("click", onBackdropClick);
    };
  }, [onClose]);

  if (!open) return null;
  return (
    <dialog
      ref={ref}
      aria-labelledby={titleId}
      className="w-full max-w-md border-0 bg-transparent p-0 backdrop:bg-black/40"
    >
      <div className="rounded-xl bg-white p-6 shadow-xl">
        <h2 id={titleId} className="mb-4 text-lg font-semibold text-gray-900">{title}</h2>
        {children}
      </div>
    </dialog>
  );
}

export function ErrorText({ children }: Readonly<{ children: ReactNode }>) {
  if (!children) return null;
  return (
    <p className="rounded-lg bg-red-50 px-3 py-2 text-sm text-red-700">
      {children}
    </p>
  );
}
