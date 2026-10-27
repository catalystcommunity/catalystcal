import { Route, Router } from "@solidjs/router";
import { A } from "@solidjs/router";
import { AppShell } from "~/layouts/AppShell";
import { LoginPage } from "~/pages/Login";
import { AuthCallback } from "~/pages/AuthCallback";
import { CalendarPage } from "~/pages/CalendarPage";
import { CalendarSettingsPage } from "~/pages/CalendarSettings";
import { DelegatesPage } from "~/pages/Delegates";
import { ActivityPage } from "~/pages/Activity";
import { InvitationsPage } from "~/pages/Invitations";
import { SettingsPage } from "~/pages/Settings";

function NotFound() {
  return (
    <div class="page">
      <h1>Page not found</h1>
      <p class="lede">
        This address is not part of catalystcal. <A href={"/"}>Go to your calendar</A>.
      </p>
    </div>
  );
}

export function App() {
  return (
    <Router>
      <Route path="/login" component={LoginPage} />
      <Route path="/auth/callback" component={AuthCallback} />
      <Route path="/" component={AppShell}>
        <Route path="/" component={CalendarPage} />
        <Route path="/calendars/:id" component={CalendarSettingsPage} />
        <Route path="/delegates" component={DelegatesPage} />
        <Route path="/activity" component={ActivityPage} />
        <Route path="/invitations" component={InvitationsPage} />
        <Route path="/settings" component={SettingsPage} />
        <Route path="*" component={NotFound} />
      </Route>
    </Router>
  );
}
