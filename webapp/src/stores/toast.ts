/** Short notices at the edge of the page (Zest notices). */
import { showNotice } from "@catalystcommunity/menus";

export function toast(text: string, kind: "info" | "error" = "info"): void {
  showNotice({ text, kind: kind === "error" ? "error" : "success" });
}
