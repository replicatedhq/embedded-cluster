import plugin from "tailwindcss/plugin";
import defaultTheme from "tailwindcss/defaultTheme";

/** @type {import('tailwindcss').Config} */
export default {
  content: ["./src/index.html", "./src/**/*.{js,jsx,ts,tsx}"],
  corePlugins: {
    preflight: true
  },
  plugins: [
    plugin(function ({ addVariant }) {
      addVariant("is-enabled", "&:not([disabled])");
      addVariant("is-disabled", "&[disabled]");
    }),
    // eslint-disable-next-line @typescript-eslint/no-var-requires
    require("@tailwindcss/forms")({ strategy: "class" })
  ]
};