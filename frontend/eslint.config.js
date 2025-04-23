import js from "@eslint/js";
import globals from "globals";
import tseslint from "typescript-eslint";
import pluginVue from "eslint-plugin-vue";
import { defineConfig } from "eslint/config";


export default defineConfig([
    { files: ["**/*.{js,mjs,cjs,ts,vue}"], plugins: { js }, extends: ["js/recommended"] },
    { files: ["**/*.{js,mjs,cjs,ts,vue}"], languageOptions: { globals: globals.browser } },
    tseslint.configs.recommended,
    pluginVue.configs["flat/recommended"],
    {
      files: ["**/*.vue"],
      languageOptions: { parserOptions: { parser: tseslint.parser } },
      rules: {
          "vue/no-deprecated-slot-attribute": "off", // csc-ui uses slots
          "vue/v-on-event-hyphenation": ["error", "always", {
            "ignore": ["changeQuery", "changeValue"], // csc-ui events
          }],
          "vue/max-attributes-per-line": ["warn", {
            "singleline": { "max": 3 }
          }],
          "@typescript-eslint/no-unused-vars": ["error",
            {
              "args": "all",
              "argsIgnorePattern": "^_",
              "caughtErrors": "all",
              "caughtErrorsIgnorePattern": "^_",
              "destructuredArrayIgnorePattern": "^_",
              "varsIgnorePattern": "^_",
              "ignoreRestSiblings": true
            }
          ]
      }
    },
]);