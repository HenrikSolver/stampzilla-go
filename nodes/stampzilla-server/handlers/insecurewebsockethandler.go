package handlers

import (
	"encoding/json"
	"encoding/pem"
	"net/http"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/stampzilla/stampzilla-go/nodes/stampzilla-server/ca"
	"github.com/stampzilla/stampzilla-go/nodes/stampzilla-server/interfaces"
	"github.com/stampzilla/stampzilla-go/nodes/stampzilla-server/models"
	"github.com/stampzilla/stampzilla-go/nodes/stampzilla-server/store"
	"github.com/stampzilla/stampzilla-go/nodes/stampzilla-server/websocket"
)

type insecureWebsocketHandler struct {
	Store           *store.Store
	Config          *models.Config
	WebsocketSender websocket.Sender
	ca              *ca.CA
}

func NewInSecureWebsockerHandler(store *store.Store, config *models.Config, ws websocket.Sender, ca *ca.CA) WebsocketHandler {
	return &insecureWebsocketHandler{
		Store:           store,
		Config:          config,
		WebsocketSender: ws,
		ca:              ca,
	}
}

func (wsh *insecureWebsocketHandler) Message(s interfaces.MelodySession, msg *models.Message) error {

	// client requested certificate. We must approve manually

	if msg.Type == "certificate-signing-request" {
		var body models.Request
		json.Unmarshal(msg.Body, &body)

		cert := &strings.Builder{}
		id, _ := s.Get(websocket.KeyID.String())
		go func() {
			err := wsh.ca.CreateCertificateFromRequest(cert, id.(string), body)
			if err != nil {
				return
			}

			// send certificate to node
			err = wsh.WebsocketSender.SendToID(msg.FromUUID, "approved-certificate-signing-request", cert.String())
			if err != nil {
				return
			}

			// send ca to node
			ca := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: wsh.ca.CAX509.Raw})

			err = wsh.WebsocketSender.SendToID(msg.FromUUID, "certificate-authority", string(ca))
			if err != nil {
				return
			}
		}()
		return nil
	}

	logrus.Warn("Unsecure ws sent data: ", msg)

	return nil
}

func (wsh *insecureWebsocketHandler) Connect(s interfaces.MelodySession, r *http.Request, keys map[string]interface{}) error {
	logrus.Debug("ws handle insecure connect")
	msg, err := models.NewMessage("server-info", models.ServerInfo{
		Name:    wsh.Config.Name,
		UUID:    wsh.Config.UUID,
		TLSPort: wsh.Config.TLSPort,
		Port:    wsh.Config.Port,
	})
	if err != nil {
		return err
	}
	msg.WriteTo(s)

	return nil
}

func (wsh *insecureWebsocketHandler) Disconnect(s interfaces.MelodySession) error {
	id, _ := s.Get(websocket.KeyID.String())
	wsh.Store.RemoveRequest(id.(string), false)
	return nil
}
