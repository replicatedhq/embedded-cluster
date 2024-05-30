package util

import (
	"fmt"
	"math/big"
	"net"
)

// https://kubernetes.io/docs/concepts/services-networking/cluster-ip-allocation/#avoid-ClusterIP-conflict
func GetLowerBandIP(cidr string, index int) (net.IP, error) {
	_, ipnet, err := net.ParseCIDR(cidr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse CIDR: %w", err)
	}

	ip := ipnet.IP.To4()
	if ip == nil {
		return nil, fmt.Errorf("only IPv4 is supported")
	}
	ipInt := big.NewInt(0).SetBytes(ip)

	ones, bits := ipnet.Mask.Size()
	rangeSize := big.NewInt(0).Lsh(big.NewInt(1), uint(bits-ones))

	bandOffset := big.NewInt(0).Div(rangeSize, big.NewInt(16))
	if bandOffset.Cmp(big.NewInt(16)) < 0 {
		bandOffset.SetInt64(16)
	}

	staticBandEnd := big.NewInt(0).Add(ipInt, bandOffset)

	rangeEnd := big.NewInt(0).Add(ipInt, rangeSize)
	if staticBandEnd.Cmp(rangeEnd) > 0 {
		staticBandEnd.Set(rangeEnd)
	}

	selectedIP := big.NewInt(0).Add(ipInt, big.NewInt(int64(index+1)))
	if selectedIP.Cmp(ipInt) <= 0 || selectedIP.Cmp(staticBandEnd) > 0 {
		return nil, fmt.Errorf("index %d is out of the band range", index)
	}

	return net.IP(selectedIP.Bytes()), nil
}
