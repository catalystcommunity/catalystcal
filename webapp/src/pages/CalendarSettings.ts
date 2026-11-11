import type { Calendar, Share, ShareRole } from "@catalystcal/client";
import { calendarClient, icsClient, shareClient } from "~/data/clients";
import { CALENDARS, accessAtLeast, bumpEvents, calendarById, calendarColor, removeCalendar, upsertCalendar } from "~/stores/calendars";
import { AUTH, effectivePerson, isActing } from "~/stores/auth";
import { confirmAction } from "~/stores/dialogs";
import { toast } from "~/stores/toast";
import { displayZone } from "~/stores/prefs";
import { parseIdentity } from "~/lib/identity";
import { identityLabel, personName, samePerson } from "~/lib/people";
import { copyText, downloadText } from "~/lib/download";
import { errorMessage } from "~/transport/csilrpc";
import { navigate } from "~/app/router";
import { CalElement, define } from "~/ui/element";
import { h, setText, syncKeyed } from "~/ui/dom";
import { icon } from "~/ui/icons";
import type { CheckControl, OptionControl, PushButton, ValueControl } from "~/ui/controls";
import type { ColorPicker } from "~/components/ColorPicker";
import { zoneOptions } from "~/components/CalendarDialogs";
import "~/components/ColorPicker";

export const ROLE_LABEL: Record<ShareRole, string> = {
  free_busy: "See only free/busy",
  read: "See all event details",
  contribute: "Add events and change their own",
  write: "Make changes to events",
  manage: "Make changes and manage sharing",
};
const ROLE_OPTIONS = Object.entries(ROLE_LABEL).map(([value, label]) => ({ value, label }));

const ACCESS_LABEL: Record<string, string> = {
  owner: "You own this calendar",
  manage: "You can change events and manage sharing",
  write: "You can change events",
  contribute: "You can add events and change the events that you added",
  read: "You can see event details",
  free_busy: "You can see only free/busy time",
  none: "You have no access",
};

/** The grantee of a share: a person, or a group. */
export function shareLabel(s: Share): string {
  if (s.granteeGroupId) return `Group: ${s.granteeGroupName ?? s.granteeGroupId}`;
  return identityLabel(s.grantee, s.granteePerson);
}

export function feedUrls(feedPath: string, origin = window.location.origin): { https: string; webcal: string } {
  const https = new URL(feedPath, origin).toString();
  return { https, webcal: https.replace(/^https?:/, "webcal:") };
}

function alertBox(): HTMLElement {
  return h("p", { class: "alert", role: "alert", hidden: true });
}
function showError(el: HTMLElement, msg: string | null): void {
  el.hidden = !msg;
  el.textContent = msg ?? "";
}
function card(id: string, title: string, ...children: (Node | null)[]): HTMLElement {
  return h("section", { class: "card", "aria-labelledby": id }, h("h2", { id }, title), children);
}

export class CalendarSettingsPage extends CalElement {
  static watches = [CALENDARS, AUTH];
  static properties = { calendarId: { type: String, default: "" } };
  declare calendarId: string;

  #loaded: Calendar | null = null;
  #loadError: string | null = null;
  #loadedId: string | null = null;
  #seq = 0;
  /** The calendar that the forms show now; a new one fills them again. */
  #formFor: Calendar | null = null;

  #content!: HTMLElement;
  #loadAlert!: HTMLElement;
  #swatch!: HTMLElement;
  #title!: HTMLElement;
  #access!: HTMLElement;
  // Subscription
  #sub!: HTMLElement;
  #subUrl!: HTMLElement;
  #subRead!: HTMLElement;
  #subError!: HTMLElement;
  #subRefresh!: PushButton;
  // Details
  #name!: ValueControl;
  #tz!: OptionControl;
  #colorField!: HTMLElement;
  #color!: ColorPicker;
  #colorView!: HTMLElement;
  #description!: ValueControl;
  #detailsError!: HTMLElement;
  #detailsSave!: PushButton;
  // Sharing
  #sharing!: HTMLElement;
  #shares: Share[] = [];
  #sharesLoaded = false;
  #sharesError!: HTMLElement;
  #shareList!: HTMLElement;
  #shareEmpty!: HTMLElement;
  #who!: ValueControl;
  #role!: OptionControl;
  #shareSubmit!: PushButton;
  #shareError!: HTMLElement;
  #sharesFor: string | null = null;
  // Feed
  #feed!: HTMLElement;
  #publish!: CheckControl;
  #feedParts!: HTMLElement;
  #feedUrl!: ValueControl;
  #webcal!: HTMLAnchorElement;
  #feedBusy = false;
  #newAddress!: PushButton;
  // Import, delete
  #import!: HTMLElement;
  #importForm!: HTMLElement & { calendarId: string };
  #delete!: HTMLElement;
  #deleteBody!: HTMLElement;
  #deleteDefault!: HTMLElement;

  protected build(): void {
    this.#loadAlert = alertBox();
    this.#swatch = h("span", { class: "swatch swatch-lg" });
    this.#title = h("h1");
    this.#access = h("p", { class: "muted" });

    this.#subUrl = h("p", { class: "mono small break" });
    this.#subRead = h("p", { class: "small" });
    this.#subError = h("p", { class: "alert small", role: "alert", hidden: true });
    this.#subRefresh = h("push-button", { onClick: () => void this.#refresh() }, icon("repeat"), " Read it now") as PushButton;
    this.#sub = card(
      "sub-h",
      "Subscription",
      h("p", { class: "small muted" }, "catalystcal reads this address again from time to time. You cannot change these events here."),
      this.#subUrl,
      this.#subRead,
      this.#subError,
      h("div", null, this.#subRefresh),
    );

    this.#name = h("text-field", { label: "Name", maxlength: "256" }) as ValueControl;
    this.#tz = h("combo-box", { label: "Time zone", editable: true }) as OptionControl;
    this.#color = h("cal-color-picker", { label: "Calendar color" }) as ColorPicker;
    this.#colorView = h("span", { class: "swatch swatch-lg" });
    this.#colorField = h("div", { class: "field" }, h("span", { class: "field-label" }, "Color"), this.#color, this.#colorView);
    this.#description = h("text-area", { label: "Description" }) as ValueControl;
    this.#detailsError = alertBox();
    this.#detailsSave = h("push-button", { type: "submit", variant: "primary" }, "Save details") as PushButton;
    const details = card(
      "gen-h",
      "Details",
      h(
        "form",
        { class: "stack", onSubmit: (e: SubmitEvent) => void this.#saveDetails(e) },
        h("div", { class: "grid-2" }, h("div", { class: "field" }, this.#name), h("div", { class: "field" }, this.#tz)),
        this.#colorField,
        h("div", { class: "field" }, this.#description),
        this.#detailsError,
        h("div", null, this.#detailsSave),
      ),
    );

    this.#sharesError = alertBox();
    this.#shareList = h("ul", { class: "list" });
    this.#shareEmpty = h("p", { class: "muted small", hidden: true }, "Not shared with anyone.");
    this.#who = h("text-field", { label: "Share with", placeholder: "user@domain", onInput: () => this.#renderShareForm() }) as ValueControl;
    this.#role = h("combo-box", { label: "Access", value: "read" }) as OptionControl;
    this.#role.options = ROLE_OPTIONS;
    this.#shareSubmit = h("push-button", { type: "submit", variant: "primary" }, "Share") as PushButton;
    this.#shareError = alertBox();
    this.#sharing = card(
      "share-h",
      "Sharing",
      h("p", { class: "muted" }, "People you share with work under their own name. To let someone act as you, add a delegate on the Delegates page."),
      this.#sharesError,
      this.#shareList,
      this.#shareEmpty,
      h(
        "form",
        { class: "share-add", onSubmit: (e: SubmitEvent) => void this.#addShare(e) },
        h("div", { class: "field grow" }, this.#who),
        h("div", { class: "field" }, this.#role),
        this.#shareSubmit,
      ),
      this.#shareError,
    );

    this.#publish = h("check-box", { onChange: () => void this.#setFeed(this.#publish.checked) }, "Publish a secret address") as CheckControl;
    this.#feedUrl = h("text-field", { label: "Secret address", readonly: true, class: "mono grow", onFocusIn: () => (this.#feedUrl as unknown as { select(): void }).select() }) as ValueControl;
    this.#webcal = h("a", { class: "btn" }, icon("calendar"), " Open in calendar app");
    this.#newAddress = h("push-button", { onClick: () => void this.#setFeed(true) }, icon("repeat"), " Make a new address") as PushButton;
    this.#feedParts = h(
      "div",
      { class: "stack" },
      h("p", { class: "notice notice-warn", role: "note" }, h("strong", null, "Keep this address secret."), " Anyone who has it can see the events in this calendar, without signing in."),
      h(
        "div",
        { class: "hrow feed-row" },
        this.#feedUrl,
        h(
          "push-button",
          {
            onClick: async () => {
              toast((await copyText(this.#feedUrl.value)) ? "Address copied." : "Copy did not work. Select the address and copy it.");
            },
          },
          icon("copy"),
          " Copy",
        ),
      ),
      h("div", { class: "hrow" }, this.#webcal, this.#newAddress),
    );
    this.#feed = card(
      "feed-h",
      "Secret address (iCalendar feed)",
      h("p", { class: "muted" }, "Apple Calendar, Google Calendar, and Outlook can subscribe to this address and show your events."),
      this.#publish,
      this.#feedParts,
    );

    const exportCard = card(
      "exp-h",
      "Export",
      h("p", { class: "muted" }, "Download every event in this calendar as an iCalendar (.ics) file."),
      h("div", null, h("push-button", { onClick: () => void this.#export() }, icon("download"), " Export .ics")),
    );

    this.#importForm = h("cal-import-ics-form") as HTMLElement & { calendarId: string };
    this.#import = card("imp-h", "Import", this.#importForm);

    this.#deleteDefault = h("p", { class: "muted" }, "This is your default calendar. You cannot delete it.");
    this.#deleteBody = h(
      "div",
      { class: "stack" },
      h("p", { class: "muted" }, "This removes the calendar and every event in it. People it is shared with lose access."),
      h("div", null, h("push-button", { variant: "danger", onClick: () => void this.#deleteCalendar() }, icon("trash"), " Delete calendar")),
    );
    this.#delete = h("section", { class: "card card-danger", "aria-labelledby": "del-h" }, h("h2", { id: "del-h" }, "Delete calendar"), this.#deleteDefault, this.#deleteBody);

    this.#content = h(
      "div",
      { hidden: true },
      h("header", { class: "page-head" }, this.#swatch, h("div", null, this.#title, this.#access)),
      this.#sub,
      details,
      this.#sharing,
      this.#feed,
      exportCard,
      this.#import,
      this.#delete,
    );
    this.append(h("div", { class: "page" }, h("a", { href: "/", class: "back-link" }, icon("chevronLeft", 16), " Calendar"), this.#loadAlert, this.#content));
  }

  #current(): Calendar | undefined {
    return this.#loaded ?? calendarById(this.calendarId ?? "");
  }

  render(): void {
    if (this.calendarId !== this.#loadedId) {
      this.#loadedId = this.calendarId;
      this.#loaded = null;
      this.#loadError = null;
      void this.#load();
    }
    const c = this.#current();
    showError(this.#loadAlert, this.#loadError && !c ? `This calendar did not load: ${this.#loadError}` : null);
    this.#content.hidden = !c;
    if (!c) return;
    const canManage = accessAtLeast(c.access, "manage");
    const canWrite = accessAtLeast(c.access, "write");
    const local = c.kind === "local";

    this.#swatch.style.setProperty("--sw", calendarColor(c));
    setText(this.#title, c.name);
    setText(
      this.#access,
      (ACCESS_LABEL[c.access] ?? c.access) +
        (!samePerson(c.owner, effectivePerson()) ? ` · owned by ${personName(c.owner)}` : "") +
        (c.accessPath === "delegation" ? " · through a delegation" : ""),
    );

    this.#sub.hidden = c.kind !== "subscription";
    setText(this.#subUrl, c.sourceUrl ?? "");
    setText(this.#subRead, c.lastSyncedAt ? `Last read ${new Date(c.lastSyncedAt).toLocaleString()}` : "Not read yet.");
    showError(this.#subError, c.syncError ? `The last read failed: ${c.syncError}` : null);
    this.#subRefresh.parentElement!.hidden = !canManage;

    if (c !== this.#formFor) {
      // A new calendar object: fill the details form again.
      this.#formFor = c;
      this.#name.value = c.name;
      const tz = c.timezone ?? displayZone();
      this.#tz.options = zoneOptions(tz);
      this.#tz.value = tz;
      this.#color.value = c.color ?? "";
      this.#description.value = c.description ?? "";
    }
    this.#name.disabled = !canManage;
    this.#tz.disabled = !canManage;
    this.#description.disabled = !canManage;
    this.#color.hidden = !canManage;
    this.#colorView.hidden = canManage;
    this.#colorView.style.setProperty("--sw", calendarColor(c));
    this.#detailsSave.parentElement!.hidden = !canManage;

    this.#sharing.hidden = !(canManage && local);
    if (canManage && local && this.#sharesFor !== c.calendarId) {
      this.#sharesFor = c.calendarId;
      void this.#loadShares();
    }
    this.#renderShares();
    this.#renderShareForm();

    this.#feed.hidden = !(canManage && local);
    this.#publish.checked = !!c.feedPath;
    this.#publish.disabled = this.#feedBusy;
    this.#newAddress.disabled = this.#feedBusy;
    this.#feedParts.hidden = !c.feedPath;
    if (c.feedPath) {
      const u = feedUrls(c.feedPath);
      this.#feedUrl.value = u.https;
      this.#webcal.href = u.webcal;
    }

    this.#import.hidden = !(canWrite && local);
    if (this.#importForm.calendarId !== c.calendarId) this.#importForm.calendarId = c.calendarId;

    this.#delete.hidden = !(c.access === "owner" && !isActing());
    this.#deleteDefault.hidden = !c.isDefault;
    this.#deleteBody.hidden = c.isDefault;
  }

  async #load(): Promise<void> {
    const id = this.calendarId;
    const seq = ++this.#seq;
    try {
      const c = await calendarClient.getCalendar(id);
      if (seq !== this.#seq || !this.isActive()) return;
      this.#loaded = c;
      upsertCalendar(c);
    } catch (e) {
      if (seq !== this.#seq || !this.isActive()) return;
      this.#loadError = errorMessage(e);
    }
    this.render();
  }

  #apply(c: Calendar): void {
    this.#loaded = c;
    upsertCalendar(c);
    this.render();
  }

  async #refresh(): Promise<void> {
    const c = this.#current();
    if (!c) return;
    this.#subRefresh.disabled = true;
    try {
      const next = await calendarClient.refreshSubscription(c.calendarId);
      bumpEvents();
      toast("Subscription read again.");
      if (this.isActive()) this.#apply(next);
    } catch (e) {
      toast(errorMessage(e), "error");
    } finally {
      this.#subRefresh.disabled = false;
    }
  }

  async #saveDetails(e: SubmitEvent): Promise<void> {
    e.preventDefault();
    const c = this.#current();
    if (!c) return;
    const name = this.#name.value.trim();
    if (!name) return showError(this.#detailsError, "Give the calendar a name.");
    this.#detailsSave.disabled = true;
    showError(this.#detailsError, null);
    try {
      const next = await calendarClient.updateCalendar({
        calendarId: c.calendarId,
        name,
        color: this.#color.value || undefined,
        timezone: this.#tz.value,
        description: this.#description.value,
      });
      toast("Calendar saved.");
      if (this.isActive()) this.#apply(next);
    } catch (err) {
      if (this.isActive()) showError(this.#detailsError, errorMessage(err));
    } finally {
      this.#detailsSave.disabled = false;
    }
  }

  async #loadShares(): Promise<void> {
    const id = this.#sharesFor;
    if (!id) return;
    try {
      const r = await shareClient.listShares(id);
      if (!this.isActive() || id !== this.#sharesFor) return;
      this.#shares = r.shares ?? [];
      showError(this.#sharesError, null);
    } catch (e) {
      if (!this.isActive() || id !== this.#sharesFor) return;
      this.#shares = [];
      showError(this.#sharesError, `Shares did not load: ${errorMessage(e)}`);
    }
    this.#sharesLoaded = true;
    this.#renderShares();
  }

  #renderShares(): void {
    syncKeyed(
      this.#shareList,
      this.#shares,
      (s) => s.shareId,
      (s) => {
        const role = h("combo-box", { class: "share-role", "aria-label": `Access for ${shareLabel(s)}`, value: s.role }) as OptionControl;
        role.options = ROLE_OPTIONS;
        role.addEventListener("change", () => void this.#changeRole(s.shareId, role.value as ShareRole));
        return h(
          "li",
          null,
          h(
            "span",
            { class: "grow" },
            h("span", { class: "truncate", style: { display: "block" } }, shareLabel(s)),
            !s.granteePerson && !s.granteeGroupId ? h("span", { class: "small muted" }, "Has not signed in to catalystcal yet") : null,
          ),
          role,
          h("button", { type: "button", class: "icon-btn", "aria-label": `Stop sharing with ${shareLabel(s)}`, onClick: () => void this.#removeShare(s.shareId) }, icon("trash", 16)),
        );
      },
      (el, s) => {
        const role = el.querySelector("combo-box") as OptionControl;
        if (role.value !== s.role) role.value = s.role;
      },
    );
    this.#shareEmpty.hidden = !(this.#sharesLoaded && this.#sharesError.hidden && this.#shares.length === 0);
  }

  #renderShareForm(): void {
    this.#shareSubmit.disabled = !this.#who.value.trim();
  }

  async #addShare(e: SubmitEvent): Promise<void> {
    e.preventDefault();
    const c = this.#current();
    if (!c) return;
    const ref = parseIdentity(this.#who.value, effectivePerson()?.domain);
    if (!ref) return showError(this.#shareError, "Enter the person as user@domain (their handle or account id at their LinkKeys domain).");
    this.#shareSubmit.disabled = true;
    showError(this.#shareError, null);
    try {
      await shareClient.putShare({ shareId: "", calendarId: c.calendarId, grantee: ref, role: this.#role.value as ShareRole, createdAt: "" });
      if (!this.isActive()) return;
      this.#who.value = "";
      await this.#loadShares();
      toast("Shared.");
    } catch (err) {
      if (this.isActive()) showError(this.#shareError, errorMessage(err));
    } finally {
      if (this.isActive()) this.#renderShareForm();
    }
  }

  async #changeRole(shareId: string, role: ShareRole): Promise<void> {
    const s = this.#shares.find((x) => x.shareId === shareId);
    if (!s || s.role === role) return;
    try {
      const updated = await shareClient.putShare({
        shareId: s.shareId,
        calendarId: s.calendarId,
        grantee: s.grantee,
        granteeGroupId: s.granteeGroupId,
        role,
        createdAt: s.createdAt,
      });
      if (!this.isActive()) return;
      this.#shares = this.#shares.map((x) => (x.shareId === shareId ? updated : x));
      this.#renderShares();
      toast("Access changed.");
    } catch (err) {
      toast(errorMessage(err), "error");
      if (this.isActive()) void this.#loadShares();
    }
  }

  async #removeShare(shareId: string): Promise<void> {
    const c = this.#current();
    const s = this.#shares.find((x) => x.shareId === shareId);
    if (!c || !s) return;
    const ok = await confirmAction({ title: "Stop sharing", message: `Stop sharing “${c.name}” with ${shareLabel(s)}?`, confirmLabel: "Stop sharing", danger: true });
    if (!ok) return;
    try {
      await shareClient.deleteShare(shareId);
      if (!this.isActive()) return;
      this.#shares = this.#shares.filter((x) => x.shareId !== shareId);
      this.#renderShares();
    } catch (err) {
      toast(errorMessage(err), "error");
    }
  }

  async #setFeed(enabled: boolean): Promise<void> {
    const c = this.#current();
    if (!c) return;
    if (enabled && c.feedPath) {
      const ok = await confirmAction({ title: "Make a new address", message: "The current address stops working. Apps that use it stop getting updates.", confirmLabel: "Make a new address" });
      if (!ok) {
        if (this.isActive()) this.render();
        return;
      }
    }
    this.#feedBusy = true;
    this.render();
    try {
      const next = await calendarClient.setFeed({ calendarId: c.calendarId, enabled });
      toast(enabled ? "Secret address ready." : "Secret address turned off.");
      if (this.isActive()) {
        this.#loaded = next;
        upsertCalendar(next);
      }
    } catch (e) {
      toast(errorMessage(e), "error");
    } finally {
      this.#feedBusy = false;
      if (this.isActive()) this.render();
    }
  }

  async #export(): Promise<void> {
    const c = this.#current();
    if (!c) return;
    try {
      const doc = await icsClient.exportCalendar({ calendarId: c.calendarId });
      downloadText(doc.filename || `${c.name}.ics`, doc.ics);
    } catch (e) {
      toast(errorMessage(e), "error");
    }
  }

  async #deleteCalendar(): Promise<void> {
    const c = this.#current();
    if (!c) return;
    const ok = await confirmAction({ title: "Delete calendar", message: `Delete “${c.name}” and all of its events? You cannot undo this.`, confirmLabel: "Delete calendar", danger: true });
    if (!ok) return;
    try {
      await calendarClient.deleteCalendar(c.calendarId);
      removeCalendar(c.calendarId);
      bumpEvents();
      toast(`Deleted “${c.name}”.`);
      navigate("/", { replace: true });
    } catch (e) {
      toast(errorMessage(e), "error");
    }
  }
}
define("cal-calendar-settings", CalendarSettingsPage);
