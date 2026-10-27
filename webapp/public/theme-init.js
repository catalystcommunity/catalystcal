// Set the theme before first paint. The key matches src/stores/theme.ts.
(function () {
  try {
    var t = localStorage.getItem("catalystcal.theme") || "system";
    if (t === "system") t = matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light";
    document.documentElement.setAttribute("data-theme", t);
  } catch (_) {}
})();
