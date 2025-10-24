/**
 * Type overrides for auto-generated API types.
 *
 * The issue: swaggo v2 doesn't infer optional from `omitempty` tags, so all
 * fields are marked as required by default. We can't add `validate:"optional"`
 * tags to external kotskinds types, so we override them here.
 *
 * TODO this is a temporary file until either:
 *  - https://github.com/swaggo/swag/issues/2040 this issue gets solved in swaggo v2
 *  - we address the type generation inconsistencies in kotskinds directly so that fields with omitempty are considered optional, e.g. https://github.com/replicatedhq/kotskinds/blob/98c7bdd50dafd5430a03c54480157c5c2f097ac5/apis/kots/v1beta1/config_types.go#L33-L64
 *
 */

import type { components } from "./api";

// Override ConfigGroup to mark fields with omitempty as optional
export type ConfigGroup = {
  name: string;
  title: string;
  description?: string;
  when?: string;
  items: ConfigItem[];
};

// Override ConfigItem to mark fields with omitempty as optional
export type ConfigItem = {
  name: string;
  type: string;
  title: string;
  help_text?: string;
  recommended?: boolean;
  default?: string;
  value?: string;
  data?: string;
  error?: string;
  multi_value?: string[];
  readonly?: boolean;
  write_once?: boolean;
  when?: string;
  multiple?: boolean;
  hidden?: boolean;
  affix?: string;
  required?: boolean;
  items?: ConfigChildItem[];
  filename?: string;
  repeatable?: boolean;
  minimumCount?: number;
  countByGroup?: { [key: string]: number };
  templates?: RepeatTemplate[];
  valuesByGroup?: components["schemas"]["v1beta1.ValuesByGroup"];
  validation?: ConfigItemValidation;
};

// Override ConfigChildItem to mark fields with omitempty as optional
export type ConfigChildItem = {
  name: string;
  title: string;
  recommended?: boolean;
  default?: string;
  value?: string;
};

// Override ConfigItemValidation to mark fields with omitempty as optional
export type ConfigItemValidation = {
  regex?: RegexValidator;
};

// Override RegexValidator - no fields have omitempty, so all required
export type RegexValidator = {
  message: string;
  pattern: string;
};

// Override RepeatTemplate to mark fields with omitempty as optional
export type RepeatTemplate = {
  apiVersion: string;
  kind: string;
  name: string;
  namespace?: string;
  yamlPath?: string;
};

// Export the corrected AppConfig type that uses our overridden types
export type AppConfig = {
  groups: ConfigGroup[];
};
