/** Stroke icons, 24px grid. Decorative: always aria-hidden. */
import type { JSX } from "solid-js";

// Factories, not elements: a JSX element is one DOM node, and sharing it
// between two icons would move it.
const paths: Record<string, () => JSX.Element> = {
  chevronLeft: () => <path d="M15 18l-6-6 6-6" />,
  chevronRight: () => <path d="M9 18l6-6-6-6" />,
  chevronDown: () => <path d="M6 9l6 6 6-6" />,
  plus: () => <path d="M12 5v14M5 12h14" />,
  search: () => (
    <>
      <circle cx="11" cy="11" r="7" />
      <path d="M20 20l-3.5-3.5" />
    </>
  ),
  menu: () => <path d="M4 6h16M4 12h16M4 18h16" />,
  close: () => <path d="M6 6l12 12M18 6L6 18" />,
  check: () => <path d="M5 12.5l4.5 4.5L19 7.5" />,
  calendar: () => (
    <>
      <rect x="3.5" y="5" width="17" height="15.5" rx="2.5" />
      <path d="M3.5 10h17M8 3v4M16 3v4" />
    </>
  ),
  users: () => (
    <>
      <circle cx="9" cy="8.5" r="3.5" />
      <path d="M2.5 20c.6-3.6 3.3-5.5 6.5-5.5s5.9 1.9 6.5 5.5" />
      <path d="M16 5.2a3.5 3.5 0 010 6.6M18 14.8c1.9.7 3.2 2.4 3.5 5.2" />
    </>
  ),
  activity: () => <path d="M3 12h4l3-7 4 14 3-7h4" />,
  inbox: () => (
    <>
      <path d="M3.5 13.5l2.7-7.4A2 2 0 018.1 4.8h7.8a2 2 0 011.9 1.3l2.7 7.4" />
      <path d="M3.5 13.5V18a2 2 0 002 2h13a2 2 0 002-2v-4.5h-5l-1.5 2.5h-4L8.5 13.5z" />
    </>
  ),
  settings: () => (
    <>
      <circle cx="12" cy="12" r="3" />
      <path d="M19.4 15a1.7 1.7 0 00.3 1.8l.1.1a2 2 0 11-2.8 2.8l-.1-.1a1.7 1.7 0 00-1.8-.3 1.7 1.7 0 00-1 1.5V21a2 2 0 11-4 0v-.1a1.7 1.7 0 00-1.1-1.5 1.7 1.7 0 00-1.8.3l-.1.1a2 2 0 11-2.8-2.8l.1-.1a1.7 1.7 0 00.3-1.8 1.7 1.7 0 00-1.5-1H3a2 2 0 110-4h.1a1.7 1.7 0 001.5-1.1 1.7 1.7 0 00-.3-1.8l-.1-.1a2 2 0 112.8-2.8l.1.1a1.7 1.7 0 001.8.3H9a1.7 1.7 0 001-1.5V3a2 2 0 114 0v.1a1.7 1.7 0 001 1.5 1.7 1.7 0 001.8-.3l.1-.1a2 2 0 112.8 2.8l-.1.1a1.7 1.7 0 00-.3 1.8V9a1.7 1.7 0 001.5 1H21a2 2 0 110 4h-.1a1.7 1.7 0 00-1.5 1z" />
    </>
  ),
  logout: () => <path d="M15 4h3a2 2 0 012 2v12a2 2 0 01-2 2h-3M10 16l-4-4 4-4M6 12h10" />,
  copy: () => (
    <>
      <rect x="8" y="8" width="12" height="12" rx="2" />
      <path d="M16 8V6a2 2 0 00-2-2H6a2 2 0 00-2 2v8a2 2 0 002 2h2" />
    </>
  ),
  download: () => <path d="M12 4v11M7 10l5 5 5-5M5 20h14" />,
  upload: () => <path d="M12 20V9M7 14l5-5 5 5M5 4h14" />,
  link: () => <path d="M10 14a4 4 0 005.7 0l3-3a4 4 0 00-5.7-5.7l-1 1M14 10a4 4 0 00-5.7 0l-3 3a4 4 0 005.7 5.7l1-1" />,
  trash: () => <path d="M4 7h16M10 11v6M14 11v6M6 7l1 12a2 2 0 002 2h6a2 2 0 002-2l1-12M9 7V4h6v3" />,
  edit: () => <path d="M4 20h4L19 9a2.8 2.8 0 00-4-4L4 16v4zM13.5 6.5l4 4" />,
  repeat: () => <path d="M17 2l3 3-3 3M20 5H8a4 4 0 00-4 4v1M7 22l-3-3 3-3M4 19h12a4 4 0 004-4v-1" />,
  pin: () => (
    <>
      <path d="M12 21s-6.5-5.6-6.5-11a6.5 6.5 0 0113 0c0 5.4-6.5 11-6.5 11z" />
      <circle cx="12" cy="10" r="2.3" />
    </>
  ),
  video: () => (
    <>
      <rect x="3" y="6" width="13" height="12" rx="2.5" />
      <path d="M16 10.5l5-3v9l-5-3" />
    </>
  ),
  clock: () => (
    <>
      <circle cx="12" cy="12" r="8.5" />
      <path d="M12 7.5V12l3 2" />
    </>
  ),
  eye: () => (
    <>
      <path d="M2.5 12S6 5.5 12 5.5 21.5 12 21.5 12 18 18.5 12 18.5 2.5 12 2.5 12z" />
      <circle cx="12" cy="12" r="2.8" />
    </>
  ),
  eyeOff: () => <path d="M3 3l18 18M10.6 5.6A9.6 9.6 0 0112 5.5c6 0 9.5 6.5 9.5 6.5a17 17 0 01-3 3.8M6.6 6.6A17 17 0 002.5 12s3.5 6.5 9.5 6.5a9 9 0 004.4-1.1M9.9 9.9a2.8 2.8 0 004.2 4.2" />,
  lock: () => (
    <>
      <rect x="5" y="10.5" width="14" height="10" rx="2" />
      <path d="M8 10.5V8a4 4 0 018 0v2.5" />
    </>
  ),
  globe: () => (
    <>
      <circle cx="12" cy="12" r="8.5" />
      <path d="M3.5 12h17M12 3.5c2.4 2.5 3.5 5.3 3.5 8.5s-1.1 6-3.5 8.5c-2.4-2.5-3.5-5.3-3.5-8.5s1.1-6 3.5-8.5z" />
    </>
  ),
  bell: () => <path d="M6 16V11a6 6 0 0112 0v5l1.5 2h-15L6 16zM10 20.5a2.2 2.2 0 004 0" />,
  swap: () => <path d="M7 4L3 8l4 4M3 8h13M17 20l4-4-4-4M21 16H8" />,
  sun: () => (
    <>
      <circle cx="12" cy="12" r="4" />
      <path d="M12 2.5v2M12 19.5v2M2.5 12h2M19.5 12h2M5.3 5.3l1.4 1.4M17.3 17.3l1.4 1.4M5.3 18.7l1.4-1.4M17.3 6.7l1.4-1.4" />
    </>
  ),
  moon: () => <path d="M20 14.5A8 8 0 019.5 4a8 8 0 1010.5 10.5z" />,
  feed: () => <path d="M5 19a1 1 0 100-.01M4 11a9 9 0 019 9M4 4a16 16 0 0116 16" />,
  info: () => (
    <>
      <circle cx="12" cy="12" r="8.5" />
      <path d="M12 11v5M12 7.5v.5" />
    </>
  ),
};

export type IconName = keyof typeof paths;

export function Icon(props: { name: IconName; size?: number; class?: string }) {
  return (
    <svg
      class={props.class}
      width={props.size ?? 18}
      height={props.size ?? 18}
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      stroke-width="1.7"
      stroke-linecap="round"
      stroke-linejoin="round"
      aria-hidden="true"
    >
      {paths[props.name]()}
    </svg>
  );
}
