package template

import (
	"encoding/base64"
	"fmt"
	"slices"

	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	"github.com/replicatedhq/kotskinds/multitype"
)

type ResolvedConfigItem struct {
	// Effective is the final resolved value following priority: user value > config value > config default
	// This is what ConfigOption functions return and what gets used in templates
	Effective string

	// UserValue is the user-provided value for the config item (if it exists)
	UserValue string

	// Value is the templated result of the config item's "value" field (if it exists)
	// This represents the config-defined value after template processing
	Value string

	// Default is the templated result of the config item's "default" field (if it exists)
	// This represents the config-defined default after template processing
	Default string

	// Filename is the filename of the "file" type config item (if it exists)
	Filename string

	// Processed indicates whether this item has been processed in the current execution
	// Used for cache invalidation - false means it's from a previous execution
	Processed bool
}

// templateConfigItems processes each config item in the engine's config by templating its value and default fields.
// It returns a copy of the config with all items processed.
func (e *Engine) templateConfigItems() (*kotsv1beta1.Config, error) {
	cfg := e.config.DeepCopy()

	for i := range cfg.Spec.Groups {
		for j := range cfg.Spec.Groups[i].Items {
			resolved, err := e.resolveConfigItem(cfg.Spec.Groups[i].Items[j].Name)
			if err != nil {
				return nil, err
			}

			// Apply user value if it exists, otherwise use the templated config value (but not the default)
			var value string
			if resolved.UserValue != "" {
				value = resolved.UserValue
			} else if resolved.Value != "" {
				value = resolved.Value
			}

			cfg.Spec.Groups[i].Items[j].Value = multitype.FromString(value)
			cfg.Spec.Groups[i].Items[j].Default = multitype.FromString(resolved.Default)
		}
	}
	return cfg, nil
}

func (e *Engine) configOption(name string) (string, error) {
	e.recordDependency(name)

	resolved, err := e.resolveConfigItem(name)
	if err != nil {
		return "", fmt.Errorf("resolve config item: %w", err)
	}
	return resolved.Effective, nil
}

func (e *Engine) configOptionData(name string) (string, error) {
	e.recordDependency(name)

	resolved, err := e.resolveConfigItem(name)
	if err != nil {
		return "", fmt.Errorf("resolve config item: %w", err)
	}

	// Base64 decode for file content
	decoded, err := base64.StdEncoding.DecodeString(resolved.Effective)
	if err != nil {
		return "", fmt.Errorf("decode base64 value: %w", err)
	}
	return string(decoded), nil
}

func (e *Engine) configOptionEquals(name, expected string) (bool, error) {
	e.recordDependency(name)

	resolved, err := e.resolveConfigItem(name)
	if err != nil {
		return false, fmt.Errorf("resolve config item: %w", err)
	}
	return resolved.Effective == expected, nil
}

func (e *Engine) configOptionNotEquals(name, expected string) (bool, error) {
	e.recordDependency(name)

	resolved, err := e.resolveConfigItem(name)
	if err != nil {
		// NOTE: this is parity from KOTS but I would expect this to return true
		return false, fmt.Errorf("resolve config item: %w", err)
	}
	return resolved.Effective != expected, nil
}

func (e *Engine) configOptionFilename(name string) (string, error) {
	e.recordDependency(name)

	resolved, err := e.resolveConfigItem(name)
	if err != nil {
		return "", fmt.Errorf("resolve config item: %w", err)
	}
	return resolved.Filename, nil
}

// resolveConfigItem processes a config item and returns its resolved values. It determines:
// 1. The effective value - the final value used in templates determined by following priority: user value > config value > config default
// 2. The templated value - the templated result of the item's "value" field
// 3. The templated default - the templated result of the item's "default" field
// 4. The filename - the filename of the "file" type config item (if it exists)
func (e *Engine) resolveConfigItem(name string) (*ResolvedConfigItem, error) {
	// Check if we have a cached value
	if cacheVal, ok := e.getItemCacheValue(name); ok {
		return cacheVal, nil
	}

	// Check for circular dependency
	if slices.Contains(e.stack, name) {
		return nil, fmt.Errorf("circular dependency detected for %s", name)
	}

	// Track resolution path for dependency discovery
	e.stack = append(e.stack, name)
	defer func() {
		if len(e.stack) > 0 {
			e.stack = e.stack[:len(e.stack)-1]
		}
	}()

	// Find the config item definition
	configItem := e.findConfigItem(name)
	if configItem == nil {
		return nil, fmt.Errorf("config item %s not found", name)
	}

	var effectiveValue, templatedValue, templatedDefault string

	// Template the value field if present
	if configItem.Value.String() != "" {
		val, err := e.processTemplate(configItem.Value.String())
		if err != nil {
			return nil, fmt.Errorf("template value for %s: %w", name, err)
		}
		templatedValue = val
	}

	// Template the default field if present
	if configItem.Default.String() != "" {
		val, err := e.processTemplate(configItem.Default.String())
		if err != nil {
			return nil, fmt.Errorf("template default for %s: %w", name, err)
		}
		templatedDefault = val
	}

	// Priority: user value > config value > config default
	userVal, exists := e.configValues[name]
	if exists {
		effectiveValue = userVal.Value
	} else if templatedValue != "" {
		effectiveValue = templatedValue
	} else {
		effectiveValue = templatedDefault
	}

	// Cache the result and mark as processed
	resolved := ResolvedConfigItem{
		Effective: effectiveValue,
		UserValue: userVal.Value,
		Value:     templatedValue,
		Default:   templatedDefault,
		Filename:  e.getItemFilename(configItem),
		Processed: true,
	}
	e.cache[name] = resolved

	return &resolved, nil
}

func (e *Engine) getItemCacheValue(name string) (*ResolvedConfigItem, bool) {
	// Check if we have a cached value
	if cacheVal, exists := e.cache[name]; exists {
		// If already processed in this execution, use it
		if cacheVal.Processed {
			return &cacheVal, true
		}

		// Value is from previous execution - check if still valid
		if !e.shouldInvalidateItem(name) {
			// Still valid - mark as processed and use it
			cacheVal.Processed = true
			e.cache[name] = cacheVal
			return &cacheVal, true
		}

		// Value is stale - remove from cache
		delete(e.cache, name)
	}

	return nil, false
}

// shouldInvalidateItem checks if a cached item should be invalidated
func (e *Engine) shouldInvalidateItem(name string) bool {
	// Check if this item's user value changed
	if e.configValueChanged(name) {
		return true
	}

	// Recursively check if any dependencies should be invalidated
	for _, dep := range e.depsTree[name] {
		if e.shouldInvalidateItem(dep) {
			return true
		}
	}

	return false
}

// configValueChanged checks if a config item's user value changed
func (e *Engine) configValueChanged(itemName string) bool {
	prevVal, prevExists := e.prevConfigValues[itemName]
	currentVal, currentExists := e.configValues[itemName]

	if prevExists != currentExists {
		return true
	}

	return prevVal.Value != currentVal.Value
}

func (e *Engine) getItemFilename(configItem *kotsv1beta1.ConfigItem) string {
	// Priority: user value
	if userVal, exists := e.configValues[configItem.Name]; exists {
		return userVal.Filename
	}

	// Do not use the config item's filename for KOTS parity

	// If still empty, return empty string
	return ""
}

func (e *Engine) findConfigItem(name string) *kotsv1beta1.ConfigItem {
	for _, group := range e.config.Spec.Groups {
		for _, item := range group.Items {
			if item.Name == name {
				return &item
			}
		}
	}
	return nil
}
