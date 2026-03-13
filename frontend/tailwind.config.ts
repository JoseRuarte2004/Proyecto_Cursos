import type { Config } from "tailwindcss";

export default {
  content: ["./index.html", "./src/**/*.{ts,tsx}"],
  theme: {
    extend: {
      colors: {
        ink: "#0f172a",
        sky: "#eef6ff",
        mist: "#f8fafc",
        brand: {
          DEFAULT: "#145af2",
          dark: "#0d3fd9",
          soft: "#dbe8ff",
        },
        accent: "#0f9d86",
        peach: "#ffedd5",
      },
      fontFamily: {
        heading: ["Sora", "ui-sans-serif", "system-ui"],
        body: ["Manrope", "ui-sans-serif", "system-ui"],
      },
      boxShadow: {
        glow: "0 24px 60px rgba(20, 90, 242, 0.16)",
        card: "0 18px 40px rgba(15, 23, 42, 0.08)",
      },
      backgroundImage: {
        "hero-grid":
          "radial-gradient(circle at top left, rgba(20,90,242,0.16), transparent 28%), radial-gradient(circle at 85% 18%, rgba(15,157,134,0.14), transparent 24%), linear-gradient(180deg, rgba(255,255,255,0.96), rgba(239,246,255,0.88))",
      },
    },
  },
  plugins: [],
} satisfies Config;
