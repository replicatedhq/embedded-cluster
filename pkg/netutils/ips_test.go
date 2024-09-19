package netutils

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFirstValidAddress(t *testing.T) {
	req := require.New(t)

	// no specified interface
	got, err := FirstValidAddress("")
	req.NoError(err)
	fmt.Printf("got ip address: %s\n", got)
	req.NotEmpty(got)

	// invalid interface
	got, err = FirstValidAddress("does-not-exist")
	req.Error(err)
	req.Empty(got)
}

func TestFirstValidIPNet(t *testing.T) {
	req := require.New(t)
	got, err := FirstValidIPNet("")
	req.NoError(err)
	fmt.Printf("got network: %s, got ip: %s\n", got.String(), got.IP.String())
	req.NotNil(got)
}
