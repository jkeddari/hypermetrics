(() => {
  const root = document.documentElement;
  const media = window.matchMedia("(prefers-color-scheme: dark)");

  function savedTheme() {
    try {
      return localStorage.getItem("theme");
    } catch {
      return null;
    }
  }

  function applyTheme(theme) {
    const dark = theme === "dark";
    root.classList.toggle("dark", dark);
    root.style.colorScheme = dark ? "dark" : "light";
    window.dispatchEvent(new CustomEvent("themechange", { detail: { theme } }));
  }

  applyTheme(savedTheme() || (media.matches ? "dark" : "light"));

  document.addEventListener("click", (event) => {
    if (!event.target.closest("[data-theme-toggle]")) return;
    const theme = root.classList.contains("dark") ? "light" : "dark";
    try {
      localStorage.setItem("theme", theme);
    } catch {}
    applyTheme(theme);
  });

  media.addEventListener("change", (event) => {
    if (!savedTheme()) applyTheme(event.matches ? "dark" : "light");
  });
})();
