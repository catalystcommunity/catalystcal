import { For, Show } from "solid-js";
import { SWATCHES } from "~/stores/calendars";

/** Swatch radio group. `allowNone` adds "use the calendar color". */
export function ColorPicker(props: { value: string; onChange: (c: string) => void; label: string; allowNone?: boolean }) {
  return (
    <div class="swatches" role="radiogroup" aria-label={props.label}>
      <Show when={props.allowNone}>
        <button type="button" role="radio" class="swatch-btn swatch-none" aria-checked={!props.value} aria-label="Calendar color" title="Calendar color" onClick={() => props.onChange("")} />
      </Show>
      <For each={SWATCHES}>
        {(c) => (
          <button
            type="button"
            role="radio"
            class="swatch-btn"
            style={{ "--sw": c }}
            aria-checked={props.value.toLowerCase() === c}
            aria-label={c}
            title={c}
            onClick={() => props.onChange(c)}
          />
        )}
      </For>
      <label class="swatch-custom" title="Pick any color">
        <span class="sr-only">Any color</span>
        <input type="color" value={props.value || SWATCHES[0]} onInput={(e) => props.onChange(e.currentTarget.value)} />
      </label>
    </div>
  );
}
