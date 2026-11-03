import tsParser from "@typescript-eslint/parser";
import tsPlugin from "@typescript-eslint/eslint-plugin";
import sveltePlugin from "eslint-plugin-svelte";
import svelteParser from "svelte-eslint-parser";

export default [
  // 1. Global Plugin Registration
  {
    plugins: {
      "@typescript-eslint": tsPlugin,
      "svelte": sveltePlugin,
    }
  },

  // Vendored shadcn-svelte components (src/lib/components/ui) are maintained
  // upstream; don't hold them to the app's strict type-aware rules.
  {
    ignores: ["src/lib/components/ui/**"],
  },

  // 2. Strict Type-Aware Rules for TypeScript (Applied to both .ts and .svelte scripts)
  {
    files: ["src/**/*.ts", "src/**/*.svelte"],
    languageOptions: {
      parser: tsParser,
      sourceType: "module",
      parserOptions: {
        projectService: true,
        tsconfigRootDir: import.meta.dirname,
        extraFileExtensions: [".svelte"],
      },
    },
    rules: {
      // Inject standard recommended & strict type-aware rulesets
      ...tsPlugin.configs["recommended-type-checked"].rules,
      ...tsPlugin.configs["strict-type-checked"].rules,

      // --- STRICT MEMORY & RESILIENCE RULES ---
      
      // Forces you to explicitly typed structures instead of escaping with 'any'
      "@typescript-eslint/no-explicit-any": "error", 
      
      // Catches unhandled or dangling Promises (preventing background tasks from getting stuck in memory)
      "@typescript-eslint/no-floating-promises": "error",
      
      // Prevents passing an async function where a synchronous one is expected (e.g., inside certain timers/event handlers)
      "@typescript-eslint/no-misused-promises": "error",

      // Warns when dynamically deleting properties via the `delete` keyword, which slows down V8 engine optimization
      "@typescript-eslint/no-dynamic-delete": "warn",

      // Enforce strict variable cleanup (ignoring only arguments prefixed with an underscore)
      "no-unused-vars": "off", 
      "@typescript-eslint/no-unused-vars": ["error", { "argsIgnorePattern": "^_", "varsIgnorePattern": "^_" }],

      // --- Project-specific relaxations (noise vs. real bugs) ---
      // Generated Wails bindings mark everything non-nullable, but the Go
      // backend can still return zero/omitted fields at runtime, so the
      // defensive `?.` and `??` throughout the app are intentional.
      "@typescript-eslint/no-unnecessary-condition": "off",
      // `() => someVoidCall()` is a common idiom in the Svelte handlers.
      "@typescript-eslint/no-confusing-void-expression": "off",
      // Interpolating numbers into user-facing strings is fine here.
      "@typescript-eslint/restrict-template-expressions": ["error", { "allowNumber": true }],
    }
  },

  // 3. Strict Svelte 5 Linting
  {
    files: ["src/**/*.svelte"],
    languageOptions: {
      parser: svelteParser,
      parserOptions: {
        parser: tsParser,
      },
    },
    rules: {
      // Inherit default recommended rules safely
      ...sveltePlugin.configs["recommended"].rules,

      // --- SVELTE 5 MEMORY MANAGEMENT ---
      "svelte/valid-compile": "error",
      "svelte/no-ignored-unsubscribe": "error",
      
      // Stops reactive loops where a state variable updates itself endlessly inside an effect
      "svelte/infinite-reactive-loop": "error",
    },
  },

  // 4. Svelte-only relaxation: the no-unsafe-* family needs fully resolved
  // types, which svelte-eslint-parser cannot always produce (it reports
  // "type could not be resolved" even for plain DOM APIs such as
  // getBoundingClientRect). svelte-check owns the real type checking of
  // .svelte files; these rules stay enabled for plain .ts sources.
  {
    files: ["src/**/*.svelte"],
    rules: {
      "@typescript-eslint/no-unsafe-assignment": "off",
      "@typescript-eslint/no-unsafe-argument": "off",
      "@typescript-eslint/no-unsafe-call": "off",
      "@typescript-eslint/no-unsafe-member-access": "off",
      "@typescript-eslint/no-unsafe-return": "off",
    },
  }
];
