import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Button, Modal } from "./ui";

// legacyCopy copies text via a temporary textarea selection; works in insecure contexts.
function legacyCopy(text: string): boolean {
  try {
    const ta = document.createElement("textarea");
    ta.value = text;
    ta.style.position = "fixed";
    ta.style.opacity = "0";
    document.body.appendChild(ta);
    ta.focus();
    ta.select();
    const ok = document.execCommand("copy");
    ta.remove();
    return ok;
  } catch {
    return false;
  }
}

// Shows a one-time recovery code. The user must confirm they have saved it, since it is
// the only way to recover their vault if they forget their password.
export function RecoveryCodeDialog({
  code,
  onClose,
}: Readonly<{
  code: string;
  onClose: () => void;
}>) {
  const [copied, setCopied] = useState(false);
  const { t } = useTranslation();

  // navigator.clipboard is only available in secure contexts (HTTPS/localhost). Fall back
  // to a textarea + execCommand("copy") so copying works over plain HTTP too.
  const copy = async () => {
    let ok = false;
    try {
      if (navigator.clipboard?.writeText) {
        await navigator.clipboard.writeText(code);
        ok = true;
      }
    } catch {
      ok = false;
    }
    if (!ok) {
      ok = legacyCopy(code);
    }
    if (ok) {
      setCopied(true);
      setTimeout(() => setCopied(false), 1500);
    }
  };

  return (
    <Modal open title={t("recoveryDialog.title")}>
      <div className="space-y-4">
        <p className="text-sm text-gray-600">{t("recoveryDialog.body")}</p>
        <div className="rounded-lg border border-amber-300 bg-amber-50 p-4 text-center font-mono text-base tracking-wide break-all text-amber-900">
          {code}
        </div>
        <div className="flex gap-2">
          <Button variant="secondary" className="flex-1" onClick={copy}>
            {copied ? t("recoveryDialog.copied") : t("recoveryDialog.copy")}
          </Button>
          <Button className="flex-1" onClick={onClose}>
            {t("recoveryDialog.acknowledge")}
          </Button>
        </div>
      </div>
    </Modal>
  );
}
