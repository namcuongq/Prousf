# HIVPN
For some reason We needed to write my own VPN. And my reason is because of my company. I want to have full internet access. HiVPN 
will help to bypass the firewall to go out to the internet.

# How it works

| My computer |<---------->TUN interface<--------->| websocket  |<----------->VPN server<----------->|  internet  |

# Support operating system

* [x] linux (vpn client and server)
* [x] Windows (only vpn client)

Features:

* HTTP Websocket
* All data is encrypted
* Easy to use CLI

# Use
## Server Mode(VPN Server)

***Only support linux***

Setting route
```
echo 1 > /proc/sys/net/ipv4/ip_forward
# Masquerade outgoing traffic
iptables -t nat -A POSTROUTING -o eth0 -j MASQUERADE
# Allow return traffic
iptables -A INPUT -i eth0 -m state --state RELATED,ESTABLISHED -j ACCEPT
```
Edit config.toml
```
Server         = "103.123.98.95:443" # server address
Address        = "172.16.0.1/24"  # ip address of the server
MTU            = 1500
TTL            = 30
# List user
Users = [
	{Username = "user", Password = "password", Ipaddress = "172.16.0.13/24"},
]
```

Run server
```
hivpn -S --config config.toml
```

## Client Mode(VPN Client)
### Linux & Windows

***Note***

On Windows need download [Wintun](https://www.wintun.net/). Download and unzip the file. The executable program needs to work with wintun.dll Put in the same directory

Edit config.toml
```
Server         = "103.123.98.95:443" # server public ip address
Address        = "172.16.0.10/24" # the ip address is configured in the config file on the server
DefaultGateway = "172.16.0.1" # ip address of the server(ip vpn)
MTU            = 1500
TTL            = 30
User           = "user" # the username is configured in the config file on the server
Pass           = "password" # the password is configured in the config file on the server
Whitelist 	   = [] # list of addresses not allowed to go through vpn. Only support CIDR. Example: "10.0.0.1/24"
Blacklist 	   = [] # blocked address list. Only suppory ip address not support CIDP. Example: "10.0.0.12"
Incognito      = false # developing
```

Run client
```
hivpn --config config.toml
```

## TODO

* [ ] dhcp
* [ ] support other protocols that are not blocked by firewall
* [ ] Add user dynamic
