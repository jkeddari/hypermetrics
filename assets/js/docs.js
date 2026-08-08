Scalar.createApiReference("#app", {
  url: "/openapi.yaml",
  theme: "default",
  darkMode: document.documentElement.classList.contains("dark"),
  layout: "modern",
  showDeveloperTools: "never",
  hideClientButton: true,
  hideModels: true,
  documentDownloadType: "none",
  defaultOpenFirstTag: false,
  withDefaultFonts: false,
  agent: {
    disabled: true,
  },
  telemetry: false,
  customCss: `
    :root {
      --scalar-font: ui-sans-serif, system-ui, sans-serif;
      --scalar-font-code: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
    }
    .light-mode {
      --scalar-color-accent: #58b400;
      --scalar-background-accent: #58b40014;
    }
    .dark-mode {
      --scalar-color-accent: #8bdc45;
      --scalar-background-accent: #8bdc451a;
    }
    .scalar-mcp-layer {
      display: none !important;
    }
  `,
  defaultHttpClient: {
    targetKey: "shell",
    clientKey: "curl",
  },
  metaData: {
    title: "Hypermetrics API Reference",
    description: "Hyperliquid market and wallet data for builders.",
  },
});

window.addEventListener("themechange", (event) => {
  const scalarRoot = document.querySelector("#app .light-mode, #app .dark-mode");
  if (!scalarRoot) return;
  const dark = event.detail.theme === "dark";
  scalarRoot.classList.toggle("dark-mode", dark);
  scalarRoot.classList.toggle("light-mode", !dark);
});
