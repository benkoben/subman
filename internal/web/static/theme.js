(() => {
  const storageKey = "subman-theme";
  const root = document.documentElement;
  const buttons = document.querySelectorAll("[data-theme-toggle]");

  const preferredTheme = () => {
    if (root.dataset.theme === "light" || root.dataset.theme === "dark") {
      return root.dataset.theme;
    }
    return window.matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light";
  };

  const updateButtons = () => {
    const dark = preferredTheme() === "dark";
    const nextLabel = dark ? "Light mode" : "Dark mode";
    buttons.forEach((button) => {
      button.setAttribute("aria-label", `Switch to ${nextLabel.toLowerCase()}`);
      button.querySelectorAll("[data-theme-label]").forEach((label) => {
        label.textContent = nextLabel;
      });
      button.querySelectorAll("[data-theme-icon]").forEach((icon) => {
        icon.textContent = dark ? "☀" : "◐";
      });
    });
  };

  buttons.forEach((button) => {
    button.addEventListener("click", () => {
      const next = preferredTheme() === "dark" ? "light" : "dark";
      root.dataset.theme = next;
      try {
        localStorage.setItem(storageKey, next);
      } catch (_) {
        // The selected scheme still applies for this page when storage is unavailable.
      }
      updateButtons();
    });
  });

  window.matchMedia("(prefers-color-scheme: dark)").addEventListener("change", () => {
    if (!root.dataset.theme) updateButtons();
  });
  updateButtons();
})();
