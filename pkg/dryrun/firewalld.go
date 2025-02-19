package dryrun

import (
	"context"
)

type FirewalldUtil struct {
}

func (f *FirewalldUtil) IsFirewalldActive(ctx context.Context) (bool, error) {
	return true, nil
}

func (f *FirewalldUtil) FirewallCmdExists(ctx context.Context) (bool, error) {
	return true, nil
}
