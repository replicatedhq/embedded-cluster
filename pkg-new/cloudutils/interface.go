package cloudutils

var _c Interface

func init() {
	Set(New())
}

func Set(c Interface) {
	_c = c
}

type Interface interface {
	TryDiscoverPublicIP() string
}

func TryDiscoverPublicIP() string {
	return _c.TryDiscoverPublicIP()
}
