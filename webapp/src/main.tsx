import { render } from "solid-js/web";
import "@fontsource-variable/schibsted-grotesk";
import "@fontsource-variable/bricolage-grotesque";
import "~/styles/index.css";
import { App } from "~/App";
import { applyTheme, watchSystemTheme } from "~/stores/theme";
import { setRestoring, useSession } from "~/stores/auth";
import { loadMe, resumeSession } from "~/lib/session";
import { loadCalendars } from "~/stores/calendars";

applyTheme();
watchSystemTheme();

// Boot: with a stored bearer, load /me and calendars. Without one, try to
// resume the browser session from its cookie before showing sign-in.
async function boot(): Promise<void> {
  if (location.pathname.startsWith("/auth/")) return;
  if (!useSession()()) {
    setRestoring(true);
    try {
      await resumeSession();
    } finally {
      setRestoring(false);
    }
  }
  if (useSession()()) {
    await loadMe();
    await loadCalendars();
  }
}
void boot();

render(() => <App />, document.getElementById("root")!);
