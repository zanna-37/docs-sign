import { useRef, useState, type ReactNode } from "react";

// Dropzone wraps content and accepts files dropped anywhere over it, showing an overlay
// while dragging. A drag counter avoids flicker when moving over child elements.
export function Dropzone({
  onFiles,
  label = "Drop files to upload",
  children,
}: {
  onFiles: (files: File[]) => void;
  label?: string;
  children: ReactNode;
}) {
  const [dragging, setDragging] = useState(false);
  const counter = useRef(0);

  return (
    <div
      className="relative"
      onDragEnter={(e) => {
        e.preventDefault();
        counter.current += 1;
        setDragging(true);
      }}
      onDragOver={(e) => e.preventDefault()}
      onDragLeave={(e) => {
        e.preventDefault();
        counter.current -= 1;
        if (counter.current <= 0) setDragging(false);
      }}
      onDrop={(e) => {
        e.preventDefault();
        counter.current = 0;
        setDragging(false);
        const files = Array.from(e.dataTransfer.files || []);
        if (files.length) onFiles(files);
      }}
    >
      {children}
      {dragging && (
        <div className="pointer-events-none absolute inset-0 z-20 flex items-center justify-center rounded-xl border-2 border-dashed border-blue-400 bg-blue-50/85 text-sm font-medium text-blue-700">
          {label}
        </div>
      )}
    </div>
  );
}
