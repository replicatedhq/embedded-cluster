// src/global.d.ts
export {};

declare global {
  interface Window {
    __INITIAL_STATE__?: any;
  }

  // For Node.js test environment
  var global: {
    window: Window;
  } & typeof globalThis;
}
