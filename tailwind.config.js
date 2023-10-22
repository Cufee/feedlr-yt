module.exports = {
  content: ["./internal/templates/**/*.templ"],
  theme: {
    extend: {
      colors: {
        primary: "#e6426d",
        secondary: "#4297e6",
        accent: "#e6d042",
      },
    },
  },
  daisyui: {
    themes: [
      {
        dark: {
          ...require("daisyui/src/theming/themes")["[data-theme=dark]"],
          primary: "#e6426d",
          secondary: "#4297e6",
          accent: "#e6d042",
        },
      },
    ],
    darkTheme: "dark",
    base: true,
    styled: true,
    utils: true,
    rtl: false,
    prefix: "",
    logs: false,
  },
  plugins: [require("daisyui")],
};
