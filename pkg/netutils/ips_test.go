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
	got, err = FirstValidAddress("foo")
	req.Error(err)
	req.Contains(err.Error(), "interface foo not found or is not valid")
	req.Empty(got)
}

func TestFirstValidIPNet(t *testing.T) {
	req := require.New(t)

	// no specified interface
	got, err := FirstValidIPNet("")
	req.NoError(err)
	fmt.Printf("got network: %s, got ip: %s\n", got.String(), got.IP.String())
	req.NotNil(got)

	// invalid interface
	got, err = FirstValidIPNet("foo")
	req.Error(err)
	req.Contains(err.Error(), "interface foo not found or is not valid")
	req.Nil(got)
}
