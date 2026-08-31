package models

import "time"

type User struct {
	ID           int64
	Username     string
	PasswordHash string
	CreatedAt    time.Time
}

type Post struct {
	ID        int64
	Title     string
	Body      string
	AuthorID  int64
	Author    string
	ImagePath string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type FileRecord struct {
	ID         int64
	Filename   string
	Path       string
	Size       int64
	UploaderID int64
	CreatedAt  time.Time
}

type ChatMessage struct {
	ID        int64     `json:"id"`
	Username  string    `json:"username"`
	Message   string    `json:"message"`
	CreatedAt time.Time `json:"created_at"`
}

// SerialPortInfo décrit un port série détecté.
type SerialPortInfo struct {
	Name         string `json:"name"`
	IsUSB        bool   `json:"is_usb"`
	VID          string `json:"vid,omitempty"`
	PID          string `json:"pid,omitempty"`
	SerialNumber string `json:"serial_number,omitempty"`
	Product      string `json:"product,omitempty"`
}

// NetworkInterfaceInfo décrit une interface réseau (eth0, eth1, wlan0...).
type NetworkInterfaceInfo struct {
	Name      string   `json:"name"`
	MAC       string   `json:"mac"`
	MTU       int      `json:"mtu"`
	Flags     string   `json:"flags"`
	Addresses []string `json:"addresses"`
	IsUp      bool     `json:"is_up"`
}

type WifiInfo struct {
	Interface string `json:"interface"`
	SSID      string `json:"ssid"`
	Signal    string `json:"signal"`
	Frequency string `json:"frequency"`
	BSSID     string `json:"bssid,omitempty"`
	Raw       string `json:"raw,omitempty"`
}
