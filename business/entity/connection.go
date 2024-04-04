package entity

var (
	DefaultClientInterfaceUp = map[string][]string{
		"linux": {
			"ip addr add dev {{ .tunnel_dev }} local {{ .server_tunnel_local_ip }} remote {{ .server_tunnel_remote_ip }}",
			"ip link set dev {{ .tunnel_dev }} mtu {{ .mtu }} up",
			"ip route add {{ .server_ip }}/32 via {{ .gateway_ip }}",
			"ip route add 0.0.0.0/1 via {{ .server_tunnel_remote_ip }}",
			"ip route add 128.0.0.0/1 via {{ .server_tunnel_remote_ip }}",
		},
		"darwin": {
			"ifconfig {{ .tunnel_dev }} {{ .server_tunnel_local_ip }} {{ .server_tunnel_remote_ip }} mtu {{ .mtu }} up",
			"route add {{ .server_ip }} {{ .gateway_ip }}",
			"route add -net 0.0.0.0 -netmask 128.0.0.0 {{ .server_tunnel_remote_ip }}",
			"route add -net 128.0.0.0 -netmask 128.0.0.0 {{ .server_tunnel_remote_ip }}",
		},
		"windows": {
			"Disable-NetAdapterBinding -Name \"{{ .tunnel_dev }}\" -ComponentID ms_tcpip6",
			"Disable-NetAdapterBinding -Name \"{{ .tunnel_dev }}\" -ComponentID ms_lldp",
			"netsh interface ip set address name=\"{{ .tunnel_dev }}\" source=static addr={{ .server_tunnel_local_ip }} mask=255.255.255.0 gateway=none",
			"netsh interface ip set interface \"{{ .tunnel_dev }}\" mtu={{ .mtu }}",
			"route add {{ .server_ip }}/32 {{ .gateway_ip }}",
			"route add 0.0.0.0/0 {{ .server_tunnel_local_ip }} IF {{ .tunnel_index }}",
		},
	}

	DefaultClientInterfaceDown = map[string][]string{
		"linux": {
			"ip route del {{ .server_ip }}/32 via {{ .gateway_ip }}",
			"ip route del 0.0.0.0/1 via {{ .server_tunnel_remote_ip }}",
			"ip route del 128.0.0.0/1 via {{ .server_tunnel_remote_ip }}",
		},
		"darwin": {
			"route delete {{ .server_ip }} {{ .gateway_ip }}",
			"route delete -net 0.0.0.0 -netmask 128.0.0.0 {{ .server_tunnel_remote_ip }}",
			"route delete -net 128.0.0.0 -netmask 128.0.0.0 {{ .server_tunnel_remote_ip }}",
		},
		"windows": {
			"route delete {{ .server_ip }}/32",
		},
	}
)
