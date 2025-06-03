import plugin from "tailwindcss/plugin";
import defaultTheme from "tailwindcss/defaultTheme";

/** @type {import('tailwindcss').Config} */
export default {
  content: ["./src/index.html", "./src/**/*.{js,jsx,ts,tsx}"],
  theme: {
    colors: {
      transparent: "transparent",
      white: "#fff",
      black: "#000",
      "off-white": "#f8f8f8",
      "teal-muted-dark": "#577981",
      "teal-medium": "#097992",
      "teal-light-accent": "#e1f0f1",
      "page-bg": "#f5f8f9",
      h1: "#161616",
      gray: {
        50: "#f9fafb",
        100: "#dedede",
        200: "#c4c8ca",
        300: "#b3b3b3",
        410: "#9b9b9b",
        400: "#959595",
        500: "#717171",
        600: "#585858",
        700: "#4f4f4f",
        800: "#323232",
        900: "#2c2c2c"
      },
      amber: {
        50: "#fffbeb",
        100: "#fef3c7",
        200: "#fde68a",
        300: "#fcd34d",
        400: "#fbbf24",
        500: "#f59e0b",
        600: "#d97706",
        700: "#b45309",
        800: "#92400e",
        900: "#78350f",
        950: "#451a03"
      },
      blue: {
        50: "#ecf4fe",
        75: "#b3d2fc",
        200: "#65a4f8",
        300: "#4591f7",
        400: "#3066ad"
      },
      green: {
        50: "#e7f7f3",
        75: "#9cdfcf",
        100: "#73d2bb",
        200: "#37bf9e",
        300: "#0eb28a",
        400: "#0a7d61",
        500: "#096d54"
      },
      indigo: {
        100: "#f0f1ff",
        200: "#c2c7fd",
        300: "#a9b0fd",
        400: "#838efc",
        500: "#6a77fb",
        600: "#4a53b0",
        700: "#414999"
      },
      neutral: {
        700: "#4A4A4A"
      },
      teal: {
        300: "#4db9c0",
        400: "#38a3a8"
      },
      pink: {
        50: "#fff0f3",
        100: "#ffc1cf",
        200: "#fea7bc",
        300: "#fe819f",
        400: "#fe678b",
        500: "#b24861",
        600: "#9b3f55"
      },
      purple: {
        400: "#7242b0"
      },
      red: {
        100: "#fee2e2",
        200: "#fecaca",
        300: "#fca5a5",
        400: "#f87171",
        500: "#ef4444",
        600: "#dc2626",
        700: "#b91c1c",
        800: "#991b1b"
      },
      yellow: {
        50: "#fffce8",
        100: "#fef9c3",
        200: "#fef08a",
        300: "#fde047",
        400: "#facc15",
        500: "#eab308",
        600: "#ca8a04",
        700: "#a16207",
        800: "#854d0e"
      },
      error: "#bc4752",
      "error-xlight": "#fbedeb",
      "error-dark": "#98222d",
      "error-bright": "#f65C5C",
      "success-bright": "#38cc97",
      disabled: "#9c9c9c",
      "warning-xlight": "#FFF9F0",
      "warning-bright": "#ec8f39",
      "info-bright": "#76bbca",
      "disabled-teal": "#76a6cf"
    },
    extend: {
      borderRadius: {
        xs: "0.125rem",
        sm: "0.187rem",
        md: "0.375rem"
      },
      fontFamily: {
        inter: ["Inter", ...defaultTheme.fontFamily.sans],
        sans: ["Open Sans", ...defaultTheme.fontFamily.sans],
        poppins: ["Poppins", ...defaultTheme.fontFamily.sans],
        helvetica: ["Helvetica Neue", ...defaultTheme.fontFamily.sans]
      },
      keyframes: {
        jiggle: {
          '0%': { transform: 'translateX(0)' },
          '10%': { transform: 'translateX(-8px)' },
          '20%': { transform: 'translateX(8px)' },
          '30%': { transform: 'translateX(-6px)' },
          '40%': { transform: 'translateX(6px)' },
          '50%': { transform: 'translateX(-4px)' },
          '60%': { transform: 'translateX(4px)' },
          '70%': { transform: 'translateX(-2px)' },
          '80%': { transform: 'translateX(2px)' },
          '90%': { transform: 'translateX(-1px)' },
          '100%': { transform: 'translateX(0)' }
        }
      },
      animation: {
        jiggle: 'jiggle 0.8s ease-in-out'
      }
    }
  },
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
