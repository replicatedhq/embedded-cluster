package helpers

import "fmt"

func AddonImageFromComponentName(componentName string) string {
	return fmt.Sprintf("proxy.replicated.com/anonymous/replicated/ec-%s", componentName)
}
