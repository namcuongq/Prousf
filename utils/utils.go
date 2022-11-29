package utils

import (
	"fmt"
	"net"
	"strings"

	"github.com/google/uuid"
)

func GenUUID() string {
	return strings.ReplaceAll(uuid.New().String(), "-", "")
}

func ValidServer(server string) (string, string, error) {
	arr := strings.Split(server, ":")
	host := server
	port := "80"
	if len(arr) > 0 {
		host = arr[0]
		port = arr[1]
	}

	if checkNotIPAddress(host) {
		ips, err := net.LookupIP(host)
		if err != nil {
			return "", "", err
		}

		if len(ips) < 1 {
			return "", "", fmt.Errorf("dns lookup %s not found", host)
		}

		for _, ip := range ips {
			if ip.To4() != nil {
				return host, ip.String() + ":" + port, nil
			}
		}
	}

	return host, server, nil
}

func checkNotIPAddress(ip string) bool {
	return net.ParseIP(ip) == nil
}
