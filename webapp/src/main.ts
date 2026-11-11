import "@fontsource-variable/schibsted-grotesk";
import "@fontsource-variable/bricolage-grotesque";
import "~/styles/index.css";
import { defineZest } from "~/ui/zest";
import { applyTheme, watchSystemTheme } from "~/stores/theme";
import { session, setRestoring } from "~/stores/auth";
import { loadMe, resumeSession } from "~/lib/session";
import { loadCalendars } from "~/stores/calendars";
import { routeLinks, startRouter } from "~/app/router";
import { App } from "~/app/App";

defineZest();
applyTheme();
watchSystemTheme();

// Boot: with a stored bearer, load /me and calendars. Without one, try to
// resume the browser session from its cookie before showing sign-in.
async function boot(): Promise<void> {
  if (location.pathname.startsWith("/auth/")) return;
  if (!session()) {
    setRestoring(true);
    try {
      await resumeSession();
    } finally {
      setRestoring(false);
    }
  }
  if (session()) {
    await loadMe();
    await loadCalendars();
  }
}

// boot() sets "restoring" before its first await, so the first render
// waits for the resume and does not send the person to sign-in.
void boot();
startRouter();
routeLinks();
document.getElementById("root")!.append(new App());
