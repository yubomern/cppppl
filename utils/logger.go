package utils

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// InterfaceLogger écrit toutes les lignes échangées avec une interface
// (port série, telnet, ftp...) dans un fichier logs/<interface>_<date>_<heure>.log
type InterfaceLogger struct {
	mu   sync.Mutex
	file *os.File
	path string
}

var loggerDir = "logs"

// SetLogDir permet de changer le dossier de logs (utilisé par main.go).
func SetLogDir(dir string) {
	loggerDir = dir
	_ = os.MkdirAll(loggerDir, 0755)
}

// NewInterfaceLogger crée un nouveau fichier de log pour une interface donnée.
// Le nom suit le format: <interface>_<YYYYMMDD>_<HHMMSS>.log
func NewInterfaceLogger(interfaceName string) (*InterfaceLogger, error) {
	_ = os.MkdirAll(loggerDir, 0755)
	safe := sanitize(interfaceName)
	now := time.Now()
	filename := fmt.Sprintf("%s_%s.log", safe, now.Format("20060102_150405"))
	fullPath := filepath.Join(loggerDir, filename)

	f, err := os.OpenFile(fullPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, err
	}
	l := &InterfaceLogger{file: f, path: fullPath}
	l.writeRaw(fmt.Sprintf("=== Session ouverte sur %s le %s ===\n", interfaceName, now.Format("2006-01-02 15:04:05")))
	return l, nil
}

func sanitize(s string) string {
	out := make([]rune, 0, len(s))
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
			out = append(out, r)
		default:
			out = append(out, '_')
		}
	}
	return string(out)
}

// Log ajoute une ligne horodatée (direction "TX" ou "RX") au fichier.
func (l *InterfaceLogger) Log(direction, line string) {
	ts := time.Now().Format("2006-01-02 15:04:05.000")
	l.writeRaw(fmt.Sprintf("[%s] %s: %s\n", ts, direction, line))
}

func (l *InterfaceLogger) writeRaw(s string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.file != nil {
		_, _ = l.file.WriteString(s)
	}
}

func (l *InterfaceLogger) Path() string {
	return l.path
}

func (l *InterfaceLogger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.file != nil {
		_, _ = l.file.WriteString(fmt.Sprintf("=== Session fermée le %s ===\n", time.Now().Format("2006-01-02 15:04:05")))
		err := l.file.Close()
		l.file = nil
		return err
	}
	return nil
}
