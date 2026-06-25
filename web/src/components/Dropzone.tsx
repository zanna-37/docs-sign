import { useRef, useState, type DragEvent, type ReactNode } from "react";

// isFileDrag reports whether a drag carries external files. Internal moves (dragging a folder or
// item onto a folder) carry our custom payload instead, so the Dropzone must ignore them and let
// the folder drop targets handle them.
function isFileDrag(e: DragEvent): boolean {
  return Array.from(e.dataTransfer.types || []).includes("Files");
}

// Dropzone wraps content and accepts files dropped anywhere over it, showing an overlay
// while dragging. A drag counter avoids flicker when moving over child elements.
export function Dropzone({
  onFiles,
  label = "Drop files to upload",
  children,
}: Readonly<{
  onFiles: (files: File[]) => void;
  label?: string;
  children: ReactNode;
}>) {
  const [dragging, setDragging] = useState(false);
  const counter = useRef(0);

  return (
    <div
      className="relative min-h-[70vh]"
      onDragEnter={(e) => {
        if (!isFileDrag(e)) return;
        e.preventDefault();
        counter.current += 1;
        setDragging(true);
      }}
      onDragOver={(e) => {
        if (isFileDrag(e)) e.preventDefault();
      }}
      onDragLeave={(e) => {
        if (!isFileDrag(e)) return;
        e.preventDefault();
        counter.current -= 1;
        if (counter.current <= 0) setDragging(false);
      }}
      onDrop={(e) => {
        if (!isFileDrag(e)) return;
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
