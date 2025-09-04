export interface NextButtonConfig {
  disabled: boolean;
  onClick: () => void;
}

export interface BackButtonConfig {
  hidden: boolean;
  disabled?: boolean;
  onClick: () => void;
}
