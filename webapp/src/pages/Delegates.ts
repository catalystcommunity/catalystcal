import type { Delegation } from "@catalystcal/client";
import { delegationClient } from "~/data/clients";
import { AUTH, isActing, session } from "~/stores/auth";
import { CALENDARS, calendarById, calendars } from "~/stores/calendars";
import { confirmAction } from "~/stores/dialogs";
import { toast } from "~/stores/toast";
import { displayZone } from "~/stores/prefs";
import { SCOPE_SHORT, normalizeScopes } from "~/lib/scopes";
import { identityLabel, initials, personName } from "~/lib/people";
import { formatDate } from "~/lib/tz";
import { actAs, stopActing } from "~/lib/session";
import { errorMessage } from "~/transport/csilrpc";
import { navigate } from "~/app/router";
import { CalElement, define } from "~/ui/element";
import { fill, h, setText } from "~/ui/dom";
import { icon } from "~/ui/icons";
import { openModal } from "~/ui/controls";
import { dateFromExpiry, type DelegationForm } from "~/components/DelegationForm";
import "~/components/DelegationForm";

function state(d: Delegation): "active" | "revoked" | "expired" {
  if (d.revokedAt) return "revoked";
  if (d.expiresAt && Date.parse(d.expiresAt) <= Date.now()) return "expired";
  return "active";
}

function terms(d: Delegation): HTMLElement {
  const tz = displayZone();
  return h(
    "div",
    { class: "deleg-terms" },
    h("div", { class: "hrow", style: { gap: "4px" } }, normalizeScopes(d.scopes).map((s) => h("span", { class: "pill pill-accent" }, SCOPE_SHORT[s]))),
    h(
      "p",
      { class: "small muted" },
      (d.calendarIds?.length ? `Only ${d.calendarIds.map((id) => calendarById(id)?.name ?? "a calendar").join(", ")}` : "All calendars, now and later") +
        (d.expiresAt ? ` · until ${formatDate(dateFromExpiry(d.expiresAt, tz), { month: "short", day: "numeric", year: "numeric" })}` : ""),
    ),
    d.note ? h("p", { class: "small" }, `“${d.note}”`) : null,
  );
}

interface ListState {
  loading: boolean;
  error: string | null;
  items: Delegation[];
}

export class DelegatesPage extends CalElement {
  static watches = [AUTH, CALENDARS];
  #actingNotice!: HTMLElement;
  #actingText!: HTMLElement;
  #lists!: HTMLElement;
  #grantedError!: HTMLElement;
  #granted!: HTMLElement;
  #receivedError!: HTMLElement;
  #received!: HTMLElement;
  #grantedState: ListState = { loading: true, error: null, items: [] };
  #receivedState: ListState = { loading: true, error: null, items: [] };
  #loadedFor: string | null = null;
  #seq = 0;

  protected build(): void {
    this.#actingText = h("p");
    this.#actingNotice = h(
      "div",
      { class: "notice notice-warn", role: "note" },
      this.#actingText,
      h("div", { style: { "margin-top": "8px" } }, h("push-button", { onClick: () => stopActing().catch((e) => toast(errorMessage(e), "error")) }, "Stop acting")),
    );
    this.#grantedError = h("p", { class: "alert", role: "alert", hidden: true });
    this.#granted = h("ul", { class: "list deleg-list" });
    this.#receivedError = h("p", { class: "alert", role: "alert", hidden: true });
    this.#received = h("ul", { class: "list deleg-list" });
    this.#lists = h(
      "div",
      null,
      h(
        "section",
        { class: "card", "aria-labelledby": "granted-h" },
        h(
          "div",
          { class: "row-between" },
          h("h2", { id: "granted-h" }, "People who can act as me"),
          h("push-button", { variant: "primary", onClick: () => this.#edit(null) }, icon("plus"), " Add a delegate"),
        ),
        this.#grantedError,
        this.#granted,
      ),
      h("section", { class: "card", "aria-labelledby": "recv-h" }, h("h2", { id: "recv-h" }, "People I can act for"), this.#receivedError, this.#received),
    );
    this.append(
      h(
        "div",
        { class: "page" },
        h(
          "header",
          { class: "page-head-text" },
          h("h1", null, "Delegates"),
          h(
            "p",
            { class: "lede" },
            "A delegate acts as you: events they make and answers they give are in your name. Use sharing on a calendar instead when someone should work under their own name.",
          ),
        ),
        this.#actingNotice,
        this.#lists,
      ),
    );
  }

  render(): void {
    const acting = isActing();
    this.#actingNotice.hidden = !acting;
    this.#lists.hidden = acting;
    if (acting) {
      const who = personName(session()?.actingAs);
      setText(this.#actingText, `You are acting as ${who}. Only ${who} can see or change who acts for them.`);
    }
    const key = session() && !acting ? session()!.user.userId : null;
    if (key !== this.#loadedFor) {
      this.#loadedFor = key;
      if (key) void this.#load();
    }
    this.#renderGranted();
    this.#renderReceived();
  }

  async #load(granted = true, received = true): Promise<void> {
    const seq = ++this.#seq;
    const run = async (fetch: () => Promise<{ delegations?: Delegation[] }>, target: ListState) => {
      target.loading = true;
      try {
        const r = await fetch();
        if (seq !== this.#seq || !this.isActive()) return;
        target.items = r.delegations ?? [];
        target.error = null;
      } catch (e) {
        if (seq !== this.#seq || !this.isActive()) return;
        target.error = errorMessage(e);
      } finally {
        target.loading = false;
      }
      if (this.isActive()) this.render();
    };
    await Promise.all([
      granted ? run(() => delegationClient.listGranted({}), this.#grantedState) : null,
      received ? run(() => delegationClient.listReceived({}), this.#receivedState) : null,
    ]);
  }

  #renderGranted(): void {
    const st = this.#grantedState;
    this.#grantedError.hidden = !st.error;
    setText(this.#grantedError, st.error ? `Delegates did not load: ${st.error}` : "");
    const list = st.error ? [] : st.items.filter((d) => !d.revokedAt);
    fill(
      this.#granted,
      list.map((d) =>
        h(
          "li",
          { class: state(d) !== "active" ? "is-expired" : "" },
          h("span", { class: "avatar" }, initials(identityLabel(d.delegate, d.delegatePerson))),
          h(
            "div",
            { class: "grow" },
            h("p", { class: "deleg-name" }, identityLabel(d.delegate, d.delegatePerson), state(d) === "expired" ? h("span", { class: "pill", style: { "margin-left": "6px" } }, "Ended") : null),
            terms(d),
          ),
          h(
            "div",
            { class: "hrow deleg-actions" },
            h("push-button", { onClick: () => this.#edit(d) }, "Change"),
            h("push-button", { variant: "danger", onClick: () => void this.#revoke(d) }, "Revoke"),
          ),
        ),
      ),
      !st.loading && !st.error && list.length === 0
        ? h("li", { class: "empty-row" }, h("p", { class: "muted" }, "Nobody can act as you. Add a delegate to let an assistant or a partner manage your calendar for you."))
        : null,
    );
  }

  #renderReceived(): void {
    const st = this.#receivedState;
    this.#receivedError.hidden = !st.error;
    setText(this.#receivedError, st.error ? `This list did not load: ${st.error}` : "");
    const list = st.error ? [] : st.items;
    fill(
      this.#received,
      list.map((d) =>
        h(
          "li",
          { class: state(d) !== "active" ? "is-expired" : "" },
          h("span", { class: "avatar avatar-acting" }, initials(personName(d.principal))),
          h("div", { class: "grow" }, h("p", { class: "deleg-name" }, personName(d.principal)), terms(d)),
          state(d) === "active"
            ? h("push-button", { variant: "primary", onClick: () => void this.#use(d) }, icon("swap"), ` Act as ${personName(d.principal)}`)
            : h("span", { class: "pill" }, state(d) === "revoked" ? "Revoked" : "Ended"),
        ),
      ),
      !st.loading && !st.error && list.length === 0 ? h("li", { class: "empty-row" }, h("p", { class: "muted" }, "Nobody has made you their delegate.")) : null,
    );
  }

  #edit(d: Delegation | null): void {
    const userId = session()?.user.userId;
    const form = h("cal-delegation-form") as DelegationForm;
    form.calendars = calendars().filter((c) => c.owner.userId === userId && c.kind === "local");
    form.initial = d ?? undefined;
    const { dialog } = openModal({ label: d ? "Change delegate" : "Add a delegate", children: [form] });
    form.addEventListener("delegation-cancel", () => dialog.close("cancel"));
    form.addEventListener("delegation-saved", () => {
      dialog.close("ok");
      toast(d ? "Delegate changed." : "Delegate added.");
      if (this.isActive()) void this.#load(true, false);
    });
  }

  async #revoke(d: Delegation): Promise<void> {
    const ok = await confirmAction({
      title: "Revoke delegate",
      message: `${identityLabel(d.delegate, d.delegatePerson)} can no longer act as you. If they are acting as you now, it stops at once.`,
      confirmLabel: "Revoke",
      danger: true,
    });
    if (!ok) return;
    try {
      await delegationClient.revokeDelegation(d.delegationId);
      toast("Delegate revoked.");
      if (this.isActive()) void this.#load(true, false);
    } catch (e) {
      toast(errorMessage(e), "error");
    }
  }

  async #use(d: Delegation): Promise<void> {
    try {
      await actAs(d.principal);
      toast(`You are now acting as ${personName(d.principal)}.`);
      navigate("/");
    } catch (e) {
      toast(errorMessage(e), "error");
    }
  }
}
define("cal-delegates-page", DelegatesPage);
