package handlers

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"go.bug.st/serial"

	"goapp/models"
	"goapp/utils"
)

// --- Gestion des connexions série actives ---------------------------------

type serialSession struct {
	port   serial.Port
	name   string
	baud   int
	prompt string
	logger *utils.InterfaceLogger
	mu     sync.Mutex
}

var (
	serialSessions   = map[string]*serialSession{}
	serialSessionsMu sync.Mutex
)

// ScanSerialPorts détecte les ports série disponibles.
// 1) via go.bug.st/serial (liste réelle du système, fiable multi-OS)
// 2) en complément, vérifie explicitement l'existence de /dev/ttyUSB0..19
//    et /dev/ttyACM0..19 pour couvrir jusqu'à 20 ports USB, comme demandé.
func ScanSerialPorts(c *gin.Context) {
	var result []models.SerialPortInfo
	seen := map[string]bool{}

	if ports, err := serial.GetPortsList(); err == nil {
		for _, p := range ports {
			result = append(result, models.SerialPortInfo{Name: p, IsUSB: strings.Contains(strings.ToUpper(p), "USB") || strings.Contains(strings.ToUpper(p), "ACM") || strings.Contains(strings.ToUpper(p), "COM")})
			seen[p] = true
		}
	}

	// Complément explicite : jusqu'à 20 ports ttyUSB / ttyACM (Linux embarqué).
	for i := 0; i < 20; i++ {
		for _, prefix := range []string{"/dev/ttyUSB", "/dev/ttyACM"} {
			name := fmt.Sprintf("%s%d", prefix, i)
			if seen[name] {
				continue
			}
			if _, err := os.Stat(name); err == nil {
				result = append(result, models.SerialPortInfo{Name: name, IsUSB: true})
				seen[name] = true
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{"ports": result, "count": len(result)})
}

type serialConnectInput struct {
	Port     string `json:"port" binding:"required"`
	Baud     int    `json:"baud"`
	Prompt   string `json:"prompt"` // ex: "TT>" — si vide, valeur par défaut "TT>"
	DataBits int    `json:"data_bits"`
	StopBits int    `json:"stop_bits"` // 1 ou 2
	Parity   string `json:"parity"`    // "none", "odd", "even"
}

// ConnectSerial ouvre un port série choisi par l'utilisateur et démarre
// un fichier de log dédié <port>_<date>_<heure>.log.
func ConnectSerial(c *gin.Context) {
	var in serialConnectInput
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if in.Baud == 0 {
		in.Baud = 115200
	}
	if in.Prompt == "" {
		in.Prompt = "TT>"
	}
	if in.DataBits == 0 {
		in.DataBits = 8
	}

	parity := serial.NoParity
	switch in.Parity {
	case "odd":
		parity = serial.OddParity
	case "even":
		parity = serial.EvenParity
	}
	stopBits := serial.OneStopBit
	if in.StopBits == 2 {
		stopBits = serial.TwoStopBits
	}

	mode := &serial.Mode{
		BaudRate: in.Baud,
		DataBits: in.DataBits,
		Parity:   parity,
		StopBits: stopBits,
	}

	port, err := serial.Open(in.Port, mode)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "connexion impossible: " + err.Error()})
		return
	}
	_ = port.SetReadTimeout(2 * time.Second)

	logger, err := utils.NewInterfaceLogger(sanitizeName(in.Port))
	if err != nil {
		_ = port.Close()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "création du fichier log impossible: " + err.Error()})
		return
	}

	sess := &serialSession{port: port, name: in.Port, baud: in.Baud, prompt: in.Prompt, logger: logger}

	serialSessionsMu.Lock()
	// ferme une éventuelle session déjà ouverte sur ce port
	if old, ok := serialSessions[in.Port]; ok {
		_ = old.port.Close()
		old.logger.Close()
	}
	serialSessions[in.Port] = sess
	serialSessionsMu.Unlock()

	c.JSON(http.StatusOK, gin.H{"ok": true, "port": in.Port, "baud": in.Baud, "prompt": in.Prompt, "log_file": logger.Path()})
}

type serialCommandInput struct {
	Port    string `json:"port" binding:"required"`
	Command string `json:"command" binding:"required"`
}

// SendSerialCommand envoie une commande sur le port ouvert et lit la
// réponse jusqu'au prompt choisi (ou jusqu'à expiration du timeout).
func SendSerialCommand(c *gin.Context) {
	var in serialCommandInput
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	serialSessionsMu.Lock()
	sess, ok := serialSessions[in.Port]
	serialSessionsMu.Unlock()
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "port non connecté"})
		return
	}

	sess.mu.Lock()
	defer sess.mu.Unlock()

	toSend := in.Command
	if !strings.HasSuffix(toSend, "\r\n") {
		toSend += "\r\n"
	}
	sess.logger.Log("TX", in.Command)

	if _, err := sess.port.Write([]byte(toSend)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "écriture impossible: " + err.Error()})
		return
	}

	response, timedOut := readUntilPrompt(sess.port, sess.prompt, 3*time.Second)
	sess.logger.Log("RX", response)

	c.JSON(http.StatusOK, gin.H{"response": response, "prompt_found": !timedOut})
}

// readUntilPrompt lit depuis le port jusqu'à trouver `prompt` dans le flux
// ou jusqu'à expiration de `timeout`.
func readUntilPrompt(port serial.Port, prompt string, timeout time.Duration) (string, bool) {
	deadline := time.Now().Add(timeout)
	var sb strings.Builder
	buf := make([]byte, 256)
	for time.Now().Before(deadline) {
		n, err := port.Read(buf)
		if err != nil {
			break
		}
		if n > 0 {
			sb.Write(buf[:n])
			if strings.Contains(sb.String(), prompt) {
				return sb.String(), false
			}
		}
	}
	return sb.String(), true
}

// DisconnectSerial ferme un port série et clôt son fichier de log.
func DisconnectSerial(c *gin.Context) {
	port := c.Param("port")
	serialSessionsMu.Lock()
	sess, ok := serialSessions[port]
	if ok {
		delete(serialSessions, port)
	}
	serialSessionsMu.Unlock()

	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "port non connecté"})
		return
	}
	_ = sess.port.Close()
	sess.logger.Close()
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// ListSerialSessions renvoie les ports actuellement connectés.
func ListSerialSessions(c *gin.Context) {
	serialSessionsMu.Lock()
	defer serialSessionsMu.Unlock()
	var names []gin.H
	for name, s := range serialSessions {
		names = append(names, gin.H{"port": name, "baud": s.baud, "prompt": s.prompt, "log_file": s.logger.Path()})
	}
	c.JSON(http.StatusOK, gin.H{"sessions": names})
}

func sanitizeName(s string) string {
	s = strings.ReplaceAll(s, "/dev/", "")
	s = strings.ReplaceAll(s, "\\\\.\\", "")
	return s
}
