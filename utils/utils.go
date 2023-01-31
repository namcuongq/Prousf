package utils

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
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

func CheckUpdate(currentVersion string) {
	var data []struct {
		TagName string `json:"tag_name"`
	}
	resp, err := http.Get("https://api.github.com/repos/namcuongq/hivpn/releases")
	if err != nil {
		log.Println("check update failed", err)
		return
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Println("check update failed", err)
		return
	}

	cv := versionOrdinal(currentVersion)
	json.Unmarshal(bodyBytes, &data)
	for _, v := range data {
		if cv > versionOrdinal(v.TagName) {
			log.Println("Please update to latest version", v.TagName, fmt.Sprintf("from https://github.com/namcuongq/hivpn/releases/download/%s", v.TagName))
			break
		}
	}
}

func versionOrdinal(version string) string {
	version = strings.Replace(version, "v.", "", 1)
	version = strings.Replace(version, "v", "", 1)

	const maxByte = 1<<8 - 1
	vo := make([]byte, 0, len(version)+8)
	j := -1
	for i := 0; i < len(version); i++ {
		b := version[i]
		if '0' > b || b > '9' {
			vo = append(vo, b)
			j = -1
			continue
		}
		if j == -1 {
			vo = append(vo, 0x00)
			j = len(vo) - 1
		}
		if vo[j] == 1 && vo[j+1] == '0' {
			vo[j+1] = b
			continue
		}
		if vo[j]+1 > maxByte {
			panic("VersionOrdinal: invalid version")
		}
		vo = append(vo, b)
		vo[j]++
	}
	return string(vo)
}
