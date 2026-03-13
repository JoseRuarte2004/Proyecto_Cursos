import { clsx, type ClassValue } from "clsx";
import { twMerge } from "tailwind-merge";

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs));
}

export function formatCurrency(amount: number, currency: string) {
  try {
    return new Intl.NumberFormat("es-AR", {
      style: "currency",
      currency,
      maximumFractionDigits: 2,
    }).format(amount);
  } catch {
    return `${currency} ${amount.toFixed(2)}`;
  }
}

export function formatDate(value: string) {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }

  return new Intl.DateTimeFormat("es-AR", {
    day: "2-digit",
    month: "short",
    year: "numeric",
  }).format(date);
}

export function slugify(value: string) {
  return value
    .toLowerCase()
    .normalize("NFD")
    .replace(/[\u0300-\u036f]/g, "")
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/(^-|-$)/g, "");
}

export function isExternalMockCheckout(url: string) {
  try {
    const parsed = new URL(url);
    return parsed.hostname === "checkout.mock";
  } catch {
    return false;
  }
}

export function getVideoEmbed(url: string) {
  try {
    const parsed = new URL(url);
    const host = parsed.hostname.replace("www.", "");

    if (host.includes("youtube.com")) {
      const id = parsed.searchParams.get("v");
      if (id) {
        return {
          kind: "iframe" as const,
          src: `https://www.youtube.com/embed/${id}`,
        };
      }
    }

    if (host === "youtu.be") {
      const id = parsed.pathname.replace("/", "");
      if (id) {
        return {
          kind: "iframe" as const,
          src: `https://www.youtube.com/embed/${id}`,
        };
      }
    }

    if (host.includes("vimeo.com")) {
      const id = parsed.pathname.split("/").filter(Boolean).at(-1);
      if (id) {
        return {
          kind: "iframe" as const,
          src: `https://player.vimeo.com/video/${id}`,
        };
      }
    }

    if (/\.(mp4|webm|ogg)$/i.test(parsed.pathname)) {
      return {
        kind: "video" as const,
        src: url,
      };
    }
  } catch {
    return null;
  }

  return {
    kind: "link" as const,
    src: url,
  };
}

export function persistJSON<T>(key: string, value: T) {
  localStorage.setItem(key, JSON.stringify(value));
}

export function readJSON<T>(key: string, fallback: T): T {
  try {
    const raw = localStorage.getItem(key);
    if (!raw) {
      return fallback;
    }
    return JSON.parse(raw) as T;
  } catch {
    return fallback;
  }
}
