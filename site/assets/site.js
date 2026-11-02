// Two controls: copy buttons for command and policy blocks, and the theme
// toggle. Everything else on the site is static.
(() => {
  const blocks = document.querySelectorAll("[data-copy]");
  for (const block of blocks) {
    const pre = block.querySelector("pre");
    const label = block.querySelector(".code-label");
    if (!pre || !label || label.querySelector(".copy")) continue;

    const button = document.createElement("button");
    button.className = "copy";
    button.type = "button";
    button.textContent = "Copy";
    button.setAttribute("aria-label", "Copy to clipboard");

    button.addEventListener("click", async () => {
      try {
        await navigator.clipboard.writeText(pre.innerText);
        button.textContent = "Copied";
        button.dataset.copied = "true";
      } catch {
        button.textContent = "Select";
        const range = document.createRange();
        range.selectNodeContents(pre);
        const selection = window.getSelection();
        selection.removeAllRanges();
        selection.addRange(range);
        return;
      }
      window.setTimeout(() => {
        button.textContent = "Copy";
        delete button.dataset.copied;
      }, 1600);
    });

    label.appendChild(button);
  }
})();

// Theme. The head script has already chosen a theme before first paint; this
// keeps the toggle's label in sync, persists an explicit choice, and follows
// the system while no choice is stored.
(() => {
  const root = document.documentElement;
  const toggle = root.querySelector(".theme-toggle");
  if (!toggle) return;

  const read = (key) => {
    try {
      return window.localStorage.getItem(key);
    } catch {
      return null;
    }
  };

  const write = (key, value) => {
    try {
      window.localStorage.setItem(key, value);
    } catch {
      /* private mode: the choice lasts for the page only */
    }
  };

  const paint = (theme) => {
    const current = theme === "light" ? "light" : "dark";
    root.dataset.theme = current;
    const target = current === "light" ? "dark" : "light";
    const text = toggle.querySelector("span");
    if (text) text.textContent = target === "light" ? "Light" : "Dark";
    toggle.setAttribute("aria-label", `Switch to ${target} theme`);
    const meta = document.querySelector('meta[name="theme-color"]');
    if (meta) meta.setAttribute("content", current === "light" ? "#f2f0e9" : "#080b10");
  };

  toggle.addEventListener("click", () => {
    const next = root.dataset.theme === "light" ? "dark" : "light";
    write("gauge-theme", next);
    paint(next);
  });

  paint(root.dataset.theme);

  window.matchMedia("(prefers-color-scheme: light)").addEventListener("change", (event) => {
    if (read("gauge-theme")) return;
    paint(event.matches ? "light" : "dark");
  });
})();
