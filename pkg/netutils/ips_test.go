package netutils

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetDefaultIPAndMask(t *testing.T) {
	req := require.New(t)
	got, err := GetDefaultIPNet()
	req.NoError(err)
	fmt.Printf("got network: %s, got ip: %s\n", got.String(), got.IP.String())
	req.NotNil(got)
}
