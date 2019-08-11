package memory

import (
	"bufio"
	"bytes"
	"io"
	"time"
	"log"
	"strings"
	"io/ioutil"
	"net/mail"
	"path/filepath"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/backend/backendutil"
	"github.com/emersion/go-message"
	"github.com/emersion/go-message/textproto"
)

type Message struct {
	Uid   uint32
	Date  time.Time
	Size  uint32
	Flags []string
	Headers  []byte
	Content  []byte
	Path  string
}

func GetMessage(id uint32) (*Message, error) {
	var flags string
	msg := &Message{Uid: id}
        err := db.QueryRow("SELECT date,size,flags,headers,path FROM messages WHERE id = ?", id).Scan(&msg.Date, &msg.Size, &flags, &msg.Headers, &msg.Path)
        if err != nil {
            log.Printf(err.Error())
            return nil, err
        }

	msg.Flags = strings.Split(flags, ",")

	return msg, nil
}

func (m *Message) GetPath() (string, error) {
	pathglob := "/home/danman/Maildir/cur/" + strings.Split(m.Path, ":")[0] + "*"

        matches, err := filepath.Glob(pathglob)
	log.Printf("found path: %s", matches)
	if err != nil {
		log.Printf(err.Error())
		return "", err
	}

	return matches[0], nil
}

func (m *Message) Body() []byte {

	if len(m.Content) > 0 {
		return m.Content
	}

	path, err := m.GetPath()
	if err != nil {
		return nil
	}

	body, err := ioutil.ReadFile(path)

	if err != nil {
		log.Printf("body: %s", err.Error())
		return nil
	}

	log.Printf("body len: %d", len(body))
	m.Content = body

	return body
}

func (m *Message) entity() (*message.Entity, error) {
	body := bytes.NewReader(m.Body())
	return message.Read(body)
}

func (m *Message) headerAndBody() (textproto.Header, io.Reader, error) {
	bodyr := bytes.NewReader(m.Body())
	body := bufio.NewReader(bodyr)
	hdr, err := textproto.ReadHeader(body)
	return hdr, body, err
}

func (m *Message) Parse() (*mail.Message, error) {
	r := bytes.NewReader(m.Body())
	return mail.ReadMessage(r)
}

func (m *Message) Fetch(seqNum uint32, items []imap.FetchItem) (*imap.Message, error) {
	fetched := imap.NewMessage(seqNum, items)
	for _, item := range items {
		switch item {
		case imap.FetchEnvelope:
			hdr, _, _ := m.headerAndBody()
			fetched.Envelope, _ = backendutil.FetchEnvelope(hdr)
		case imap.FetchBody, imap.FetchBodyStructure:
			hdr, body, _ := m.headerAndBody()
			fetched.BodyStructure, _ = backendutil.FetchBodyStructure(hdr, body, item == imap.FetchBodyStructure)
			fetched.BodyStructure.Size = 1 //workaround until size parser implementation
			log.Printf("bodystruct %s", fetched.BodyStructure)
		case imap.FetchFlags:
			fetched.Flags = m.Flags
		case imap.FetchInternalDate:
			fetched.InternalDate = m.Date
		case imap.FetchRFC822Size:
			fetched.Size = m.Size
		case imap.FetchUid:
			fetched.Uid = m.Uid
		default:
			section, err := imap.ParseBodySectionName(item)
			if err != nil {
				break
			}

			body := bufio.NewReader(bytes.NewReader(m.Body()))
			hdr, err := textproto.ReadHeader(body)
			if err != nil {
				return nil, err
			}

			l, _ := backendutil.FetchBodySection(hdr, body, section)
			fetched.Body[section] = l
		}
	}

	return fetched, nil
}

func (m *Message) Match(seqNum uint32, c *imap.SearchCriteria) (bool, error) {
	e, _ := m.entity()
	return backendutil.Match(e, seqNum, m.Uid, m.Date, m.Flags, c)
}
