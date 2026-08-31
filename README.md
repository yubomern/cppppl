# goapp — Console web Gin (série, telnet, ftp, réseau/wifi, blog, chat)

Application Go (Gin) full-stack, testée et compilée avec succès dans
l'environnement de build (Go 1.22).

## Fonctionnalités

- **Authentification** : inscription / connexion / session par cookie, mots
  de passe hashés (bcrypt), SQLite3.
- **Port série** : scan jusqu'à 20 ports USB (`/dev/ttyUSB0-19`,
  `/dev/ttyACM0-19` + détection système via `go.bug.st/serial`), connexion
  avec baudrate/parité/stop bits configurables, envoi de commandes avec
  détection de prompt (`TT>` par défaut, modifiable), log dédié
  `logs/<port>_<date>_<heure>.log`.
- **Telnet** : connexion TCP brute vers `host:port`, envoi de commandes,
  détection de prompt, log dédié.
- **FTP** : connexion, listing de répertoire, upload, download, log dédié.
- **Réseau** : `ip a`, liste des interfaces (eth0, eth1, wlan0...), détail
  d'une interface précise, infos WiFi (SSID/signal via `nmcli` puis
  `iwconfig` en repli).
- **Blog** : titre + corps riche (CKEditor 5 via CDN), upload d'image
  intégré (`SimpleUploadAdapter`), CRUD complet.
- **Gestion de fichiers** : upload/liste/suppression générique.
- **Chat** : temps réel via `gorilla/websocket`, historique persistant
  en SQLite.
- **Frontend** : HTML/CSS/JS vanilla + AJAX (`fetch`), pas de framework
  lourd côté client.

## Démarrage

```bash
go mod tidy   # si besoin, dans un environnement avec accès réseau normal
go run main.go
```

Le serveur écoute sur `http://localhost:8080`. La base `app.db` et le
dossier `logs/` sont créés automatiquement au premier lancement.

## Note sur go.mod (mirrors GitHub)

Cet environnement de build avait un accès réseau restreint (liste blanche
de domaines, sans accès à `go.bug.st`, `golang.org`, `gopkg.in`,
`nullprogram.com`). Pour compiler et **valider réellement le code**, des
directives `replace` ont été ajoutées dans `go.mod` pointant vers des
mirrors GitHub équivalents (mêmes versions) :

```
replace go.bug.st/serial => github.com/bugst/go-serial v1.6.2
replace golang.org/x/crypto => github.com/golang/crypto v0.26.0
replace golang.org/x/sys => github.com/golang/sys v0.24.0
replace golang.org/x/arch => github.com/golang/arch v0.8.0
replace golang.org/x/net => github.com/golang/net v0.25.0
replace golang.org/x/text => github.com/golang/text v0.17.0
replace google.golang.org/protobuf => github.com/protocolbuffers/protobuf-go v1.34.2
replace gopkg.in/yaml.v3 => github.com/go-yaml/yaml v3.0.1+incompatible
replace gopkg.in/check.v1 => github.com/go-check/check v0.0.0-20161208181325-20d25e280405
```

**Sur une machine avec un accès Internet normal, ces `replace` ne sont pas
nécessaires** — tu peux les supprimer de `go.mod` et lancer `go mod tidy`
normalement, il utilisera les vrais modules (`go.bug.st/serial`,
`golang.org/x/...`, etc.) via le proxy Go standard.

## Structure

```
goapp/
├── main.go              # routes Gin
├── db/db.go              # init SQLite + migrations
├── models/models.go       # structs
├── middleware/auth.go      # sessions cookie
├── handlers/
│   ├── auth_handlers.go
│   ├── serial_handlers.go  # scan/connect/send port série
│   ├── telnet_handlers.go
│   ├── ftp_handlers.go
│   ├── network_handlers.go # ip a, interfaces, wifi
│   ├── blog_handlers.go    # CKEditor + upload image
│   ├── file_handlers.go
│   └── chat_handlers.go    # websocket
├── utils/logger.go         # logs interface_date_time.log
├── templates/*.html
└── static/{css,js,uploads}/
```

## Sécurité — à adapter avant toute mise en production

- `upgrader.CheckOrigin` (chat websocket) accepte actuellement toute
  origine — à restreindre.
- Le cookie de session n'est pas marqué `Secure` (adapté au HTTPS) —
  à activer derrière un reverse proxy TLS.
- Aucune limite de taux (rate limiting) sur les endpoints d'authentification.
