// Package inbox conecta al buzón IMAP dedicado y extrae, de los correos sin
// leer, los adjuntos .xml listos para que el parser de Fase 3 los procese.
package inbox

import (
	"fmt"
	"io"
	"strings"

	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"
	"github.com/emersion/go-message/mail"

	"rhix-backend/internal/config"
)

// XMLAttachment es un adjunto .xml encontrado en un correo.
type XMLAttachment struct {
	Filename string
	Content  []byte
}

// PendingEmail es un correo UNSEEN con al menos un adjunto .xml.
type PendingEmail struct {
	UID         imap.UID
	MessageID   string
	Subject     string
	Attachments []XMLAttachment
}

// Session mantiene una conexión IMAP abierta durante todo el ciclo del
// worker, para poder marcar correos como SEEN recién después de persistir
// cada recibo con éxito (Fase 3), sin reconectar por cada uno.
type Session struct {
	client *imapclient.Client
}

// Connect abre la conexión, autentica y selecciona el INBOX.
func Connect(cfg *config.Config) (*Session, error) {
	addr := fmt.Sprintf("%s:%s", cfg.ImapHost, cfg.ImapPort)

	c, err := imapclient.DialTLS(addr, nil)
	if err != nil {
		return nil, fmt.Errorf("conectando a %s: %w", addr, err)
	}

	if err := c.Login(cfg.ImapUser, cfg.ImapPassword).Wait(); err != nil {
		c.Close()
		return nil, fmt.Errorf("login IMAP: %w", err)
	}

	if _, err := c.Select("INBOX", nil).Wait(); err != nil {
		c.Close()
		return nil, fmt.Errorf("seleccionando INBOX: %w", err)
	}

	return &Session{client: c}, nil
}

// Close cierra la conexión IMAP.
func (s *Session) Close() error {
	return s.client.Close()
}

// FetchPendingXML busca los correos UNSEEN y devuelve, para cada uno que
// tenga adjuntos .xml, sus datos y adjuntos ya extraídos en memoria. No
// marca ningún correo como SEEN.
func (s *Session) FetchPendingXML() ([]PendingEmail, error) {
	searchData, err := s.client.UIDSearch(&imap.SearchCriteria{
		NotFlag: []imap.Flag{imap.FlagSeen},
	}, nil).Wait()
	if err != nil {
		return nil, fmt.Errorf("buscando correos UNSEEN: %w", err)
	}

	uids := searchData.AllUIDs()
	if len(uids) == 0 {
		return nil, nil
	}

	bodySection := &imap.FetchItemBodySection{}
	fetchOptions := &imap.FetchOptions{
		UID:         true,
		Envelope:    true,
		BodySection: []*imap.FetchItemBodySection{bodySection},
	}

	fetchCmd := s.client.Fetch(imap.UIDSetNum(uids...), fetchOptions)
	defer fetchCmd.Close()

	var pending []PendingEmail

	for {
		msg := fetchCmd.Next()
		if msg == nil {
			break
		}

		var (
			uid      imap.UID
			envelope *imap.Envelope
			body     io.Reader
		)

		for {
			item := msg.Next()
			if item == nil {
				break
			}
			switch item := item.(type) {
			case imapclient.FetchItemDataUID:
				uid = item.UID
			case imapclient.FetchItemDataEnvelope:
				envelope = item.Envelope
			case imapclient.FetchItemDataBodySection:
				body = item.Literal
			}
		}

		if body == nil {
			continue
		}

		attachments, err := extractXMLAttachments(body)
		if err != nil {
			return nil, fmt.Errorf("leyendo correo UID %d: %w", uid, err)
		}
		if len(attachments) == 0 {
			continue
		}

		email := PendingEmail{UID: uid, Attachments: attachments}
		if envelope != nil {
			email.Subject = envelope.Subject
			email.MessageID = envelope.MessageID
		}
		pending = append(pending, email)
	}

	if err := fetchCmd.Close(); err != nil {
		return nil, fmt.Errorf("comando FETCH: %w", err)
	}

	return pending, nil
}

// MarkSeen marca un correo como leído. Debe llamarse solo después de
// persistir con éxito el recibo que trae ese correo (Fase 3).
func (s *Session) MarkSeen(uid imap.UID) error {
	storeFlags := imap.StoreFlags{
		Op:    imap.StoreFlagsAdd,
		Flags: []imap.Flag{imap.FlagSeen},
	}
	return s.client.Store(imap.UIDSetNum(uid), &storeFlags, nil).Close()
}

func extractXMLAttachments(body io.Reader) ([]XMLAttachment, error) {
	mr, err := mail.CreateReader(body)
	if err != nil {
		return nil, fmt.Errorf("parseando mensaje: %w", err)
	}

	var attachments []XMLAttachment
	for {
		p, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("parseando parte del mensaje: %w", err)
		}

		header, ok := p.Header.(*mail.AttachmentHeader)
		if !ok {
			continue
		}

		filename, err := header.Filename()
		if err != nil || !strings.HasSuffix(strings.ToLower(filename), ".xml") {
			continue
		}

		content, err := io.ReadAll(p.Body)
		if err != nil {
			return nil, fmt.Errorf("leyendo adjunto %s: %w", filename, err)
		}

		attachments = append(attachments, XMLAttachment{Filename: filename, Content: content})
	}

	return attachments, nil
}
