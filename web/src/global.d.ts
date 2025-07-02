// src/global.d.ts
export { };

declare global {
  interface Window {
    __INITIAL_STATE__?: InitialState;
  }

  // Initial state is how the server can pass initial data to the client.
  interface InitialState {
    icon?: string;
    title?: string;
    installTarget?: string;
  }
}
