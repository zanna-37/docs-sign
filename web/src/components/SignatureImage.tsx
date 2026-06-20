import { useSignatureBitmap } from "../lib/signatureBitmaps";
import { SignatureCanvas } from "./SignatureCanvas";

// SignatureImage renders a signature on a canvas from an in-memory ImageBitmap, so the
// decrypted image is never an <img>/object-URL resource the browser could cache or save.
export function SignatureImage({
  id,
  alt,
  className,
}: Readonly<{
  id: string;
  alt?: string;
  className?: string;
}>) {
  const bitmap = useSignatureBitmap(id);
  if (!bitmap) {
    return <div className={`animate-pulse bg-gray-100 ${className ?? ""}`} />;
  }
  return (
    <SignatureCanvas bitmap={bitmap} className={className} ariaLabel={alt} />
  );
}
