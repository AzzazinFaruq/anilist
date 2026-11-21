// Runs before React so the page never flashes the wrong theme.
try {
  const t = localStorage.getItem("tonton-theme");
  if (t === "light" || t === "dark") document.documentElement.dataset.theme = t;
} catch {
  /* storage blocked: follow the system */
}
