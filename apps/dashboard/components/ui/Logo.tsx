export default function Logo({ compact = false }: { compact?: boolean }) {
  return (
    <span className="inline-flex items-center gap-2.5">
      <span className="grid h-9 w-9 place-items-center rounded-xl bg-gradient-to-br from-indigo-500 to-violet-600 text-white shadow-lg shadow-indigo-600/30">
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" aria-hidden="true" className="h-5 w-5">
          <path d="M3 17l5-6 4 4 6-8" />
          <path d="M18 6h-4M18 6v4" />
        </svg>
      </span>
      {!compact && (
        <span className="flex flex-col leading-none">
          <span className="text-[17px] font-bold tracking-tight text-zinc-900 dark:text-white">DevPulse</span>
          <span className="text-[11px] font-medium tracking-wide text-zinc-500 dark:text-zinc-400">Privacy analytics</span>
        </span>
      )}
    </span>
  );
}
