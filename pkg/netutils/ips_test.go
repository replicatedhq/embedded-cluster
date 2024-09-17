package netutils

import (
	"fmt"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestGetDefaultIPAndMask(t *testing.T) {
	req := require.New(t)
	got, err := GetDefaultIPAndMask()
	req.NoError(err)
	fmt.Printf("got network: %s, got ip: %s\n", got.String(), got.IP.String())
	req.NotNil(got)
}
