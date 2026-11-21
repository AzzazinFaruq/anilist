/** Stroke icons drawn on a 24px grid so they sit exactly in the middle of round buttons. */
const PATHS = {
  minus: "M6 12h12",
  plus: "M6 12h12M12 6v12",
  prev: "M14.5 6l-6 6 6 6",
  next: "M9.5 6l6 6-6 6",
};

export function Icon({ name }: { name: keyof typeof PATHS }) {
  return (
    <svg className="icon" viewBox="0 0 24 24" width="20" height="20" aria-hidden="true">
      <path d={PATHS[name]} fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  );
}
