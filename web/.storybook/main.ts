import type { StorybookConfig } from "@storybook/react-vite";

const config: StorybookConfig = {
  framework: "@storybook/react-vite",
  stories: ["../src/**/*.stories.@(ts|tsx)"],
  addons: ["@storybook/addon-docs", "@storybook/addon-a11y", "@storybook/addon-vitest"],
  staticDirs: ["./public", { from: "../../assets", to: "/assets" }],
  viteFinal: async (config) => ({
    ...config,
    base: "./",
    build: { ...config.build, manifest: false },
  }),
  // The builder reads ../vite.config.ts, including the React Compiler transform.
};
export default config;
