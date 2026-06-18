import {
  createContext,
  useCallback,
  useContext,
  useState,
  type ReactNode,
} from "react";
import { useTranslation } from "react-i18next";
import { Button, Input, Modal } from "./ui";

interface ConfirmOpts {
  title: string;
  message?: string;
  confirmLabel?: string;
  danger?: boolean;
}
interface PromptOpts {
  title: string;
  message?: string;
  defaultValue?: string;
  placeholder?: string;
  confirmLabel?: string;
}

interface DialogApi {
  confirm: (opts: ConfirmOpts) => Promise<boolean>;
  prompt: (opts: PromptOpts) => Promise<string | null>;
}

const DialogCtx = createContext<DialogApi | null>(null);

type Active =
  | null
  | { type: "confirm"; opts: ConfirmOpts; resolve: (v: boolean) => void }
  | { type: "prompt"; opts: PromptOpts; resolve: (v: string | null) => void };

export function DialogProvider({ children }: { children: ReactNode }) {
  const { t } = useTranslation();
  const [active, setActive] = useState<Active>(null);
  const [value, setValue] = useState("");

  const confirm = useCallback(
    (opts: ConfirmOpts) =>
      new Promise<boolean>((resolve) =>
        setActive({ type: "confirm", opts, resolve }),
      ),
    [],
  );

  const prompt = useCallback(
    (opts: PromptOpts) =>
      new Promise<string | null>((resolve) => {
        setValue(opts.defaultValue ?? "");
        setActive({ type: "prompt", opts, resolve });
      }),
    [],
  );

  const settle = (v: boolean | string | null) => {
    setActive((a) => {
      if (a) (a.resolve as (x: boolean | string | null) => void)(v);
      return null;
    });
  };

  return (
    <DialogCtx.Provider value={{ confirm, prompt }}>
      {children}
      {active && (
        <Modal
          open
          title={active.opts.title}
          onClose={() => settle(active.type === "confirm" ? false : null)}
        >
          {active.type === "confirm" ? (
            <div className="space-y-5">
              {active.opts.message && (
                <p className="text-sm text-gray-600">{active.opts.message}</p>
              )}
              <div className="flex justify-end gap-2">
                <Button variant="secondary" onClick={() => settle(false)}>
                  {t("common.cancel")}
                </Button>
                <Button
                  variant={active.opts.danger ? "danger" : "primary"}
                  onClick={() => settle(true)}
                >
                  {active.opts.confirmLabel ?? t("common.confirm")}
                </Button>
              </div>
            </div>
          ) : (
            <form
              className="space-y-4"
              onSubmit={(e) => {
                e.preventDefault();
                settle(value);
              }}
            >
              {active.opts.message && (
                <p className="text-sm text-gray-600">{active.opts.message}</p>
              )}
              <Input
                autoFocus
                value={value}
                placeholder={active.opts.placeholder}
                onChange={(e) => setValue(e.target.value)}
              />
              <div className="flex justify-end gap-2">
                <Button
                  type="button"
                  variant="secondary"
                  onClick={() => settle(null)}
                >
                  {t("common.cancel")}
                </Button>
                <Button type="submit">
                  {active.opts.confirmLabel ?? t("common.confirm")}
                </Button>
              </div>
            </form>
          )}
        </Modal>
      )}
    </DialogCtx.Provider>
  );
}

export function useDialog(): DialogApi {
  const ctx = useContext(DialogCtx);
  if (!ctx) throw new Error("useDialog must be used within DialogProvider");
  return ctx;
}
