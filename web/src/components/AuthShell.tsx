import type { ReactNode } from "react";
import { AppVersion } from "./AppVersion";
import { Card } from "./ui";

export function AuthShell({
  title,
  subtitle,
  children,
}: Readonly<{
  title: string;
  subtitle?: string;
  children: ReactNode;
}>) {
  return (
    <div className="flex min-h-full items-center justify-center p-6">
      <div className="w-full max-w-sm">
        <div className="mb-6 text-center">
          <div className="text-2xl font-semibold text-gray-900">
            <span className="text-blue-600">✦</span> docs-sign
          </div>
          <h1 className="mt-4 text-lg font-semibold text-gray-900">{title}</h1>
          {subtitle && <p className="mt-1 text-sm text-gray-500">{subtitle}</p>}
        </div>
        <Card className="p-6">{children}</Card>
        <p className="mt-6 text-center text-xs text-gray-400">
          <AppVersion />
        </p>
      </div>
    </div>
  );
}
