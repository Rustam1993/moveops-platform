import { type ButtonHTMLAttributes, type InputHTMLAttributes } from "react";
import { clsx } from "clsx";

export function Button(props: ButtonHTMLAttributes<HTMLButtonElement>) {
  const { className, ...rest } = props;
  return (
    <button
      className={clsx(
        "rounded-md bg-primary px-4 py-2 font-semibold text-white transition hover:opacity-90 disabled:opacity-60",
        className,
      )}
      {...rest}
    />
  );
}

export function Input(props: InputHTMLAttributes<HTMLInputElement>) {
  const { className, ...rest } = props;
  return (
    <input
      className={clsx(
        "w-full rounded-md border border-slate-300 bg-white px-3 py-2 outline-none ring-primary focus:ring-2",
        className,
      )}
      {...rest}
    />
  );
}

export function Card({ children, className }: { children: React.ReactNode; className?: string }) {
  return <div className={clsx("rounded-xl bg-card p-6 shadow-sm", className)}>{children}</div>;
}
