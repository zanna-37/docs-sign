import { useBlobUrl } from "../lib/blobUrls";

// SignatureImage renders a signature PNG fetched into an in-memory object URL (no-store),
// so the decrypted image is never written to the browser's disk cache.
export function SignatureImage({
  id,
  alt,
  className,
}: {
  id: string;
  alt?: string;
  className?: string;
}) {
  const url = useBlobUrl(`/api/signatures/${id}/image`);
  if (!url) {
    return <div className={`animate-pulse bg-gray-100 ${className ?? ""}`} />;
  }
  return <img src={url} alt={alt ?? ""} className={className} />;
}
