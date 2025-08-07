import { InitialState } from './types';
// src/global.d.ts
export { };

declare global {
  interface Window {
    __INITIAL_STATE__?: InitialState;
  }
}
