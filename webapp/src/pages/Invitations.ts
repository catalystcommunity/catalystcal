import type { Event, ParticipationStatus } from "@catalystcal/client";
import { eventClient } from "~/data/clients";
import { AUTH, effectivePerson, hasScope, isActing, session } from "~/stores/auth";
import { CALENDARS, bumpEvents, eventsVersion } from "~/stores/calendars";
import { displayZone } from "~/stores/prefs";
import { goToDate } from "~/stores/nav";
import { openDetail } from "~/stores/editor";
import { toast } from "~/stores/toast";
import { STATUS_LABEL, myAttendee } from "~/lib/eventVisual";
import { eventDate, occurrenceOf } from "~/lib/eventRef";
import { formatDate, formatTimeRange, parseTimestamp } from "~/lib/tz";
import { personName } from "~/lib/people";
import { navigate } from "~/app/router";
import { errorMessage } from "~/transport/csilrpc";
import { CalElement, define } from "~/ui/element";
import { h, setText, syncKeyed } from "~/ui/dom";
import type { PushButton } from "~/ui/controls";

const ACTIONS: { status: ParticipationStatus; label: string }[] = [
  { status: "accepted", label: "Accept" },
  { status: "tentative", label: "Maybe" },
  { status: "declined", label: "Decline" },
];

export class InvitationsPage extends CalElement {
  static watches = [AUTH, CALENDARS];
  #list: Event[] = [];
  #loaded = false;
  #answered: Record<string, ParticipationStatus> = {};
  #key = "";
  #seq = 0;
  #lede!: HTMLElement;
  #error!: HTMLElement;
  #scopeNotice!: HTMLElement;
  #pending!: HTMLUListElement;
  #pendingEmpty!: HTMLElement;
  #doneSection!: HTMLElement;
  #done!: HTMLUListElement;

  protected build(): void {
    this.#lede = h("p", { class: "lede" });
    this.#error = h("p", { class: "alert", role: "alert", hidden: true });
    this.#scopeNotice = h("p", { class: "notice", hidden: true }, "Your delegation does not let you answer invitations for this person.");
    this.#pending = h("ul", { class: "list" });
    this.#pendingEmpty = h("p", { class: "empty", hidden: true }, "You have answered every invitation.");
    this.#done = h("ul", { class: "list" });
    this.#doneSection = h("section", { class: "card", "aria-labelledby": "inv-done", hidden: true }, h("h2", { id: "inv-done" }, "Answered"), this.#done);
    this.append(
      h(
        "div",
        { class: "page" },
        h("header", { class: "page-head-text" }, h("h1", null, "Invitations"), this.#lede),
        this.#error,
        this.#scopeNotice,
        h("section", { class: "card", "aria-labelledby": "inv-new" }, h("h2", { id: "inv-new" }, "Waiting for an answer"), this.#pending, this.#pendingEmpty),
        this.#doneSection,
      ),
    );
  }

  render(): void {
    const s = session();
    if (!s) return;
    const key = `${effectivePerson()?.userId}|${eventsVersion()}`;
    if (key !== this.#key) {
      this.#key = key;
      void this.#load();
    }
    setText(
      this.#lede,
      isActing() ? `Invitations sent to ${personName(effectivePerson())}. Your answers are sent in their name.` : "Events other people invited you to.",
    );
    this.#scopeNotice.hidden = hasScope("events.respond");
    const pending = this.#list.filter((e) => this.#statusOf(e) === "needs_action");
    const done = this.#list.filter((e) => this.#statusOf(e) !== "needs_action");
    syncKeyed(this.#pending, pending, (e) => e.eventId, (e) => this.#row(e), (li, e) => this.#updateRow(li, e));
    syncKeyed(this.#done, done, (e) => e.eventId, (e) => this.#row(e), (li, e) => this.#updateRow(li, e));
    this.#pendingEmpty.hidden = !this.#loaded || pending.length > 0;
    this.#doneSection.hidden = done.length === 0;
  }

  async #load(): Promise<void> {
    const my = ++this.#seq;
    try {
      const r = await eventClient.listInvitations({});
      if (my !== this.#seq || !this.isActive()) return;
      this.#list = r.events ?? [];
      this.#answered = {};
      this.#error.hidden = true;
    } catch (e) {
      if (my !== this.#seq || !this.isActive()) return;
      this.#list = [];
      setText(this.#error, `Invitations did not load: ${errorMessage(e)}`);
      this.#error.hidden = false;
    }
    this.#loaded = true;
    this.render();
  }

  #statusOf(e: Event): ParticipationStatus {
    return this.#answered[e.eventId] ?? myAttendee({ event: e })?.status ?? "needs_action";
  }

  #when(e: Event): string {
    const tz = displayZone();
    const d = eventDate(e, tz);
    if (!d) return "";
    const day = formatDate(d, { weekday: "short", month: "short", day: "numeric", year: "numeric" });
    if (e.allDay || !e.start || !e.end) return `${day}, all day`;
    return `${day}, ${formatTimeRange(parseTimestamp(e.start), parseTimestamp(e.end), displayZone())}`;
  }

  #row(e: Event): HTMLLIElement {
    const open = () => {
      const d = eventDate(e, displayZone());
      if (d) goToDate(d, "day");
      navigate("/");
      openDetail(occurrenceOf(e, true));
    };
    return h(
      "li",
      { class: "invite" },
      h(
        "div",
        { class: "grow" },
        h("button", { type: "button", class: "link-btn invite-title", onClick: open }),
        h("p", { class: "small muted invite-when" }),
        h("p", { class: "small muted invite-where" }),
      ),
      h(
        "div",
        { class: "segmented", role: "group" },
        ACTIONS.map((a) => h("push-button", { toggle: true, "data-status": a.status, onClick: () => void this.#respond(e, a.status) }, a.label)),
      ),
    );
  }

  #updateRow(li: HTMLElement, e: Event): void {
    setText(li.querySelector(".invite-title")!, e.title);
    const parts = [this.#when(e)];
    if (e.organizer) parts.push(`from ${personName(e.organizer)}`);
    if (e.recurrence) parts.push("repeats");
    setText(li.querySelector(".invite-when")!, parts.join(" · "));
    const where = li.querySelector<HTMLElement>(".invite-where")!;
    setText(where, e.location ?? "");
    where.hidden = !e.location;
    li.querySelector(".segmented")!.setAttribute("aria-label", `Answer ${e.title}`);
    const status = this.#statusOf(e);
    const canRespond = hasScope("events.respond");
    for (const b of li.querySelectorAll<PushButton>("push-button")) {
      b.pressed = b.dataset.status === status;
      b.disabled = !canRespond;
    }
  }

  async #respond(e: Event, status: ParticipationStatus): Promise<void> {
    // A toggle button flips itself on the click; the answer decides.
    this.invalidate();
    try {
      await eventClient.respond({ eventId: e.eventId, status });
      if (!this.isActive()) return;
      this.#answered = { ...this.#answered, [e.eventId]: status };
      toast(`${STATUS_LABEL[status]}: ${e.title}`);
      bumpEvents();
    } catch (err) {
      toast(errorMessage(err), "error");
      if (this.isActive()) void this.#load();
    }
  }
}
define("cal-invitations-page", InvitationsPage);
