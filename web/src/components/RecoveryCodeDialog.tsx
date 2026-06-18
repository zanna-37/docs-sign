import { useState } from "react";
import { Button, Modal } from "./ui";

// Shows a one-time recovery code. The user must confirm they have saved it, since it is
// the only way to recover their vault if they forget their password.
export function RecoveryCodeDialog({
  code,
  onClose,
}: {
  code: string;
  onClose: () => void;
}) {
  const [copied, setCopied] = useState(false);

  const copy = async () => {
    try {
      await navigator.clipboard.writeText(code);
      setCopied(true);
      setTimeout(() => setCopied(false), 1500);
    } catch {
      /* clipboard may be unavailable; the code is still shown */
    }
  };

  return (
    <Modal open title="Save your recovery code">
      <div className="space-y-4">
        <p className="text-sm text-gray-600">
          This is the <strong>only</strong> way to recover your encrypted data if
          you forget your password. It is shown once and never again. Store it
          somewhere safe and offline.
        </p>
        <div className="rounded-lg border border-amber-300 bg-amber-50 p-4 text-center font-mono text-base tracking-wide break-all text-amber-900">
          {code}
        </div>
        <div className="flex gap-2">
          <Button variant="secondary" className="flex-1" onClick={copy}>
            {copied ? "Copied!" : "Copy"}
          </Button>
          <Button className="flex-1" onClick={onClose}>
            I have saved it
          </Button>
        </div>
      </div>
    </Modal>
  );
}
