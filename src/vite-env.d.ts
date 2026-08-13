/// <reference types="vite/client" />

interface ImportMetaEnv {
  readonly VITE_FEATURE_RENTAL_STORE?: string;
}

interface ImportMeta {
  readonly env: ImportMetaEnv;
}
