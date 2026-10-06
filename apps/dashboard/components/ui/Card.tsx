import type { ElementType, HTMLAttributes, ReactNode } from "react";

interface CardProps extends HTMLAttributes<HTMLElement> {
  children: ReactNode;
  className?: string;
  as?: ElementType;
}

export default function Card({ children, className = "", as, ...rest }: CardProps) {
  const Tag = as ?? "div";
  return (
    <Tag className={`rounded-2xl border border-zinc-200 bg-white p-5 shadow-sm shadow-zinc-950/5 dark:border-zinc-800 dark:bg-zinc-900 dark:shadow-none ${className}`} {...rest}>
      {children}
    </Tag>
  );
}
