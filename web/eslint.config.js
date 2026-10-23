import js from "@eslint/js";
import query from "@tanstack/eslint-plugin-query";
import reactHooks from "eslint-plugin-react-hooks";
import globals from "globals";
import tseslint from "typescript-eslint";

const manualHooks = ["memo", "useMemo", "useCallback"];
const externalHooks = ["useEffect", "useLayoutEffect", "useInsertionEffect"];
export default tseslint.config(
  {
    ignores: [
      "node_modules/**",
      "storybook-static/**",
      "src/lib/api/schema.d.ts",
      ".storybook/public/mockServiceWorker.js",
      ".storybook/__screenshots__/**",
    ],
  },
  js.configs.recommended,
  ...tseslint.configs.recommended,
  reactHooks.configs.flat.recommended,
  ...query.configs["flat/recommended"],
  {
    languageOptions: { globals: { ...globals.browser, ...globals.node } },
    rules: {
      "no-restricted-imports": [
        "error",
        {
          paths: [
            {
              name: "react",
              importNames: [...manualHooks, ...externalHooks],
              message:
                "Use render derivation, events, Query and React Compiler. External lifecycle hooks belong in explicit adapters.",
            },
          ],
        },
      ],
      "no-restricted-syntax": [
        "error",
        {
          selector: 'ExpressionStatement[expression.value="use no memo"]',
          message: "Compiler opt-outs need a documented, reviewed exception.",
        },
      ],
    },
  },
  {
    files: ["src/features/playback/adapters/**", "src/lib/browser/**"],
    rules: {
      "no-restricted-imports": [
        "error",
        {
          paths: [
            {
              name: "react",
              importNames: manualHooks,
              message: "React Compiler owns memoization.",
            },
          ],
        },
      ],
    },
  },
);
