export default {
  extends: ["stylelint-config-standard"],
  ignoreFiles: ["node_modules/**", ".storybook/public/**"],
  rules: {
    // Tailwind resolves bare string imports before Vite's CSS import pass.
    "import-notation": "string",
    "at-rule-no-unknown": [
      true,
      {
        ignoreAtRules: [
          "theme",
          "source",
          "utility",
          "variant",
          "custom-variant",
          "apply",
          "reference",
          "config",
          "plugin",
        ],
      },
    ],
  },
};
