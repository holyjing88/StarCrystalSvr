/// <reference types="vite/client" />

interface ImportMetaEnv {
  readonly VITE_IDIP_KEY?: string;
}

interface ImportMeta {
  readonly env: ImportMetaEnv;
}
