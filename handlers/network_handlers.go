package handlers

import (
	"net"
	"net/http"
	"os/exec"
	"regexp"
	"runtime"
	"strings"

	"github.com/gin-gonic/gin"
	"goapp/models"
)

// ListInterfaces renvoie toutes les interfaces réseau (eth0, eth1, wlan0, lo...)
// avec leurs adresses IP, en s'appuyant sur le package "net" (portable).
func ListInterfaces(c *gin.Context) {
	ifaces, err := net.Interfaces()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var result []models.NetworkInterfaceInfo
	for _, ifc := range ifaces {
		addrs, _ := ifc.Addrs()
		var addrStrs []string
		for _, a := range addrs {
			addrStrs = append(addrStrs, a.String())
		}
		result = append(result, models.NetworkInterfaceInfo{
			Name:      ifc.Name,
			MAC:       ifc.HardwareAddr.String(),
			MTU:       ifc.MTU,
			Flags:     ifc.Flags.String(),
			Addresses: addrStrs,
			IsUp:      ifc.Flags&net.FlagUp != 0,
		})
	}
	c.JSON(http.StatusOK, gin.H{"interfaces": result})
}

// RawIPA exécute `ip a` (Linux) et renvoie la sortie brute, pour affichage
// tel quel côté client (utile pour retrouver eth0/eth1 précisément).
func RawIPA(c *gin.Context) {
	if runtime.GOOS != "linux" {
		c.JSON(http.StatusOK, gin.H{"output": "commande 'ip a' disponible uniquement sous Linux sur ce serveur", "supported": false})
		return
	}
	out, err := exec.Command("ip", "a").CombinedOutput()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "output": string(out)})
		return
	}
	c.JSON(http.StatusOK, gin.H{"output": string(out), "supported": true})
}

// InterfaceDetail renvoie `ip addr show <iface>` pour une interface précise
// (eth0, eth1, wlan0...).
func InterfaceDetail(c *gin.Context) {
	name := c.Param("name")
	if runtime.GOOS != "linux" {
		c.JSON(http.StatusOK, gin.H{"output": "disponible uniquement sous Linux", "supported": false})
		return
	}
	out, err := exec.Command("ip", "addr", "show", name).CombinedOutput()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "output": string(out)})
		return
	}
	c.JSON(http.StatusOK, gin.H{"output": string(out), "supported": true})
}

// WifiInfoHandler tente de récupérer le SSID / signal du wifi courant.
// Utilise `nmcli` en priorité (NetworkManager), puis se rabat sur `iwconfig`.
func WifiInfoHandler(c *gin.Context) {
	if runtime.GOOS != "linux" {
		c.JSON(http.StatusOK, gin.H{"supported": false, "message": "wifi info disponible uniquement sous Linux"})
		return
	}

	// 1) nmcli (le plus fiable, généralement présent sur Ubuntu/Debian/RaspberryPi OS)
	if out, err := exec.Command("nmcli", "-t", "-f", "active,ssid,signal,freq,bssid,device", "dev", "wifi").CombinedOutput(); err == nil {
		var results []models.WifiInfo
		for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
			if line == "" {
				continue
			}
			parts := strings.Split(line, ":")
			if len(parts) < 6 {
				continue
			}
			if parts[0] != "yes" {
				continue // on ne garde que la connexion active
			}
			results = append(results, models.WifiInfo{
				Interface: parts[5],
				SSID:      parts[1],
				Signal:    parts[2] + "%",
				Frequency: parts[3],
				BSSID:     parts[4],
			})
		}
		if len(results) > 0 {
			c.JSON(http.StatusOK, gin.H{"supported": true, "source": "nmcli", "wifi": results})
			return
		}
	}

	// 2) iwconfig (fallback, plus ancien)
	if out, err := exec.Command("iwconfig").CombinedOutput(); err == nil {
		text := string(out)
		re := regexp.MustCompile(`(?m)^(\S+)\s+IEEE 802\.11.*?ESSID:"([^"]*)"`)
		matches := re.FindAllStringSubmatch(text, -1)
		var results []models.WifiInfo
		for _, m := range matches {
			results = append(results, models.WifiInfo{
				Interface: m[1],
				SSID:      m[2],
				Raw:       text,
			})
		}
		if len(results) > 0 {
			c.JSON(http.StatusOK, gin.H{"supported": true, "source": "iwconfig", "wifi": results})
			return
		}
		c.JSON(http.StatusOK, gin.H{"supported": true, "source": "iwconfig", "raw": text})
		return
	}

	c.JSON(http.StatusOK, gin.H{"supported": false, "message": "ni nmcli ni iwconfig disponibles sur ce système"})
}
