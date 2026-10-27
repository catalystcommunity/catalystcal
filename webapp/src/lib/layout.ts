/**
 * Layout for the time grid and the all-day / month bars.
 *
 * layoutTimed: events that overlap in time sit side by side. Events that
 * overlap, directly or through a chain, form a cluster; each cluster splits
 * its width into as many columns as it needs, and an event widens to the
 * right over columns that are free for its whole time.
 */

export interface TimedItem {
  id: string;
  /** Minutes (or any unit) from the top of the day. */
  start: number;
  end: number;
}

export interface TimedBox {
  /** Column index, from 0. */
  col: number;
  /** Columns in the cluster. */
  cols: number;
  /** How many columns this event covers (>= 1). */
  span: number;
}

/**
 * @param minLength a short event takes at least this much room, so a
 *   5-minute event and the one after it do not draw on top of each other.
 */
export function layoutTimed(items: TimedItem[], minLength = 0): Map<string, TimedBox> {
  const out = new Map<string, TimedBox>();
  const sorted = items
    .map((it) => ({ ...it, end: Math.max(it.end, it.start + minLength) }))
    .sort((a, b) => a.start - b.start || b.end - a.end || a.id.localeCompare(b.id));

  let cluster: { it: TimedItem; col: number }[] = [];
  let columnsEnd: number[] = [];
  let clusterEnd = -Infinity;

  const flush = () => {
    const cols = columnsEnd.length;
    for (const { it, col } of cluster) {
      let span = 1;
      // Widen over following columns that no clustered item overlaps.
      for (let c = col + 1; c < cols; c++) {
        const blocked = cluster.some((o) => o.col === c && o.it.start < it.end && it.start < o.it.end);
        if (blocked) break;
        span++;
      }
      out.set(it.id, { col, cols, span });
    }
    cluster = [];
    columnsEnd = [];
    clusterEnd = -Infinity;
  };

  for (const it of sorted) {
    if (it.start >= clusterEnd && cluster.length) flush();
    let col = columnsEnd.findIndex((end) => end <= it.start);
    if (col < 0) {
      col = columnsEnd.length;
      columnsEnd.push(it.end);
    } else {
      columnsEnd[col] = it.end;
    }
    cluster.push({ it, col });
    clusterEnd = Math.max(clusterEnd, it.end);
  }
  if (cluster.length) flush();
  return out;
}

export interface SpanItem {
  id: string;
  /** First day index covered (inclusive). */
  startIdx: number;
  /** Last day index covered (inclusive). */
  endIdx: number;
}

/**
 * Give multi-day bars lanes (rows) so no two bars in a lane share a day.
 * Longer bars first, then earlier, so long bars stay on top.
 */
export function assignLanes(items: SpanItem[]): Map<string, number> {
  const sorted = [...items].sort(
    (a, b) => a.startIdx - b.startIdx || b.endIdx - b.startIdx - (a.endIdx - a.startIdx) || a.id.localeCompare(b.id),
  );
  const lanes: number[][] = []; // lane -> occupied endIdx per bar; check by ranges
  const laneItems: SpanItem[][] = [];
  const out = new Map<string, number>();
  for (const it of sorted) {
    let lane = laneItems.findIndex((list) => list.every((o) => o.endIdx < it.startIdx || it.endIdx < o.startIdx));
    if (lane < 0) {
      lane = laneItems.length;
      laneItems.push([]);
      lanes.push([]);
    }
    laneItems[lane].push(it);
    out.set(it.id, lane);
  }
  return out;
}
