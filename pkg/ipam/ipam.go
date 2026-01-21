package ipam

// FIXME: netlinkwrapper or network
import (
	"test-cni-plugin/pkg/config"

	"github.com/vishvananda/netlink"
)

type IPAM struct {
	subnet string
}

func NewIPAM(config *config.IPAMConfig) IPAM {
	return IPAM{subnet: config.Subnet}
}

func (ipam *IPAM) BindNewAddr(link netlink.Link) (*netlink.Addr, error) {
	addr, err := ipam.newAddr()
	if err != nil {
		return nil, err
	}

	if err := netlink.AddrAdd(link, addr); err != nil {
		return nil, err
	}

	return addr, nil
}

func (*IPAM) newAddr() (*netlink.Addr, error) {
	// FIXME
	return netlink.ParseAddr("10.244.0.2/24")
}
