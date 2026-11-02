// Copy buttons for command and policy blocks. Everything else is static.
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
