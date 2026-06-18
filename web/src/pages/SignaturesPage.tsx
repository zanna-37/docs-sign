import { useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { api, ApiError } from "../api/client";
import type { Signature } from "../api/types";
import { Button, Card, ErrorText, Spinner } from "../components/ui";
import { SignatureImage } from "../components/SignatureImage";
import { Dropzone } from "../components/Dropzone";
import { TrashIcon } from "../components/icons";
import { formatBytes, formatDate } from "../lib/format";

const checker: React.CSSProperties = {
  backgroundColor: "#fff",
  backgroundImage:
    "linear-gradient(45deg,#eee 25%,transparent 25%),linear-gradient(-45deg,#eee 25%,transparent 25%),linear-gradient(45deg,transparent 75%,#eee 75%),linear-gradient(-45deg,transparent 75%,#eee 75%)",
  backgroundSize: "16px 16px",
  backgroundPosition: "0 0,0 8px,8px -8px,-8px 0",
};

export function SignaturesPage() {
  const { t } = useTranslation();
  const [items, setItems] = useState<Signature[] | null>(null);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const fileRef = useRef<HTMLInputElement>(null);

  const reload = async () => {
    try {
      const res = await api.get<{ signatures: Signature[] }>("/signatures");
      setItems(res.signatures ?? []);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : t("common.failedLoad"));
    }
  };

  useEffect(() => {
    void reload();
  }, []);

  const uploadFiles = async (files: File[]) => {
    setError("");
    setBusy(true);
    try {
      for (const file of files) {
        const isPng = file.type === "image/png" || /\.png$/i.test(file.name);
        if (!isPng) {
          setError(t("signatures.notPng", { name: file.name }));
          continue;
        }
        await api.upload<Signature>("/signatures", file, file.name);
      }
      await reload();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : t("common.uploadFailed"));
    } finally {
      setBusy(false);
    }
  };

  const onUpload = (e: React.ChangeEvent<HTMLInputElement>) => {
    const files = Array.from(e.target.files || []);
    e.target.value = "";
    if (files.length) void uploadFiles(files);
  };

  const rename = async (s: Signature) => {
    const name = prompt(t("signatures.renamePrompt"), s.name);
    if (!name || name === s.name) return;
    try {
      await api.patch(`/signatures/${s.id}`, { name });
      await reload();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : t("common.renameFailed"));
    }
  };

  const remove = async (s: Signature) => {
    if (!confirm(t("signatures.confirmDelete", { name: s.name }))) return;
    try {
      await api.del(`/signatures/${s.id}`);
      await reload();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : t("common.deleteFailed"));
    }
  };

  return (
    <Dropzone onFiles={uploadFiles} label={t("signatures.drop")}>
      <div className="space-y-6">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <div>
            <h1 className="text-xl font-semibold text-gray-900">
              {t("signatures.title")}
            </h1>
            <p className="text-sm text-gray-500">{t("signatures.subtitle")}</p>
          </div>
          <input
            ref={fileRef}
            type="file"
            accept="image/png"
            multiple
            className="hidden"
            onChange={onUpload}
          />
          <Button onClick={() => fileRef.current?.click()} disabled={busy}>
            {busy ? t("signatures.uploading") : t("signatures.upload")}
          </Button>
        </div>

        <ErrorText>{error}</ErrorText>

        {items === null ? (
          <Spinner />
        ) : items.length === 0 ? (
          <Card className="p-10 text-center text-sm text-gray-500">
            {t("signatures.empty")}
          </Card>
        ) : (
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3">
            {items.map((s) => (
              <Card key={s.id} className="overflow-hidden">
                <div
                  className="flex h-36 items-center justify-center p-4"
                  style={checker}
                >
                  <SignatureImage
                    id={s.id}
                    alt={s.name}
                    className="max-h-full max-w-full object-contain"
                  />
                </div>
                <div className="flex items-center justify-between gap-2 border-t border-gray-100 p-3">
                  <div className="min-w-0">
                    <p className="truncate text-sm font-medium text-gray-800">
                      {s.name}
                    </p>
                    <p className="text-xs text-gray-400">
                      {s.width}×{s.height} · {formatBytes(s.byteSize)} ·{" "}
                      {formatDate(s.createdAt)}
                    </p>
                  </div>
                  <div className="flex shrink-0 gap-1">
                    <Button variant="secondary" onClick={() => rename(s)}>
                      {t("common.rename")}
                    </Button>
                    <Button
                      variant="secondary"
                      className="px-2"
                      title={t("common.delete")}
                      aria-label={t("common.delete")}
                      onClick={() => remove(s)}
                    >
                      <TrashIcon className="h-4 w-4 text-red-600" />
                    </Button>
                  </div>
                </div>
              </Card>
            ))}
          </div>
        )}
      </div>
    </Dropzone>
  );
}
