package memory

import (
	"io/ioutil"
	"time"
	"log"
	"os"
	"path/filepath"

	"github.com/emersion/go-imap"
//	"github.com/emersion/go-imap/backend"
	"github.com/emersion/go-imap/backend/backendutil"
)

var Delimiter = "/"

type Mailbox struct {
	Subscribed bool
	Messages   []*Message

	Id   int32
	Path string
	name string
	user *User
}

func (mbox *Mailbox) Name() string {
	return mbox.name
}

func (mbox *Mailbox) Info() (*imap.MailboxInfo, error) {
	info := &imap.MailboxInfo{
		Delimiter: Delimiter,
		Name:      mbox.name,
	}
	return info, nil
}

func (mbox *Mailbox) uidNext() uint32 {
	var uid uint32
        err := db.QueryRow("SELECT MAX(message) FROM mappings WHERE mailbox = ?", mbox.Id).Scan(&uid)
        if err != nil {
            log.Printf(err.Error())
            return 0
        }

	uid++
	log.Printf("%d",uid)
	return uid
}

func (mbox *Mailbox) flags() []string {
	flagsMap := make(map[string]bool)
	for _, msg := range mbox.Messages {
		for _, f := range msg.Flags {
			if !flagsMap[f] {
				flagsMap[f] = true
			}
		}
	}

	var flags []string
	for f := range flagsMap {
		flags = append(flags, f)
	}
	return flags
}

func (mbox *Mailbox) unseenSeqNum() uint32 {
	for i, msg := range mbox.Messages {
		seqNum := uint32(i + 1)

		seen := false
		for _, flag := range msg.Flags {
			if flag == imap.SeenFlag {
				seen = true
				break
			}
		}

		if !seen {
			return seqNum
		}
	}
	return 0
}


func (mbox *Mailbox) messageCount() (uint32) {
	var count uint32
        err := db.QueryRow("SELECT count(*) FROM mappings WHERE mailbox = ?", mbox.Id).Scan(&count)
        if err != nil {
            log.Printf(err.Error())
            return 0
        }

	return count
}

func (mbox *Mailbox) Status(items []imap.StatusItem) (*imap.MailboxStatus, error) {
	status := imap.NewMailboxStatus(mbox.name, items)
	status.Flags = mbox.flags()
	status.PermanentFlags = []string{"\\*"}
	status.UnseenSeqNum = mbox.unseenSeqNum()

	for _, name := range items {
		switch name {
		case imap.StatusMessages:
			status.Messages = mbox.messageCount()
		case imap.StatusUidNext:
			status.UidNext = mbox.uidNext()
		case imap.StatusUidValidity:
			status.UidValidity = 1
		case imap.StatusRecent:
			status.Recent = 0 // TODO
		case imap.StatusUnseen:
			status.Unseen = 0 // TODO
		}
	}

	return status, nil
}

func (mbox *Mailbox) SetSubscribed(subscribed bool) error {
	mbox.Subscribed = subscribed
	return nil
}

func (mbox *Mailbox) Check() error {
	return nil
}

func (mbox *Mailbox) ListMessages(uid bool, seqSet *imap.SeqSet, items []imap.FetchItem, ch chan<- *imap.Message) error {
	defer close(ch)
	log.Printf("ListMessages %s %s %s", uid, seqSet.String(), items)

	if uid {

		//TODO: sanitize seqSet
		var muid uint32
		for muid = 1; muid < 10000000 ; muid++ {
			if !seqSet.Contains(muid){
				continue
			}
			msg, err := NewMessage(muid)
			if err != nil {
				log.Printf(err.Error())
				return err
			}

			//log.Printf("Fetch %d", muid)
			m, err := msg.Fetch(muid, items)
			if err != nil {
				continue
			}

			ch <- m
		}

	} else {

	        results, err := db.Query("SELECT messsage.id FROM mappings LEFT JOIN messages on mappings.message = message.id WHERE mailbox = ?", mbox.Id)
	        if err != nil {
	            log.Printf(err.Error())
	            return err
	        }
	
	        for results.Next() {
	            msg := &imap.Message{}
	            // for each row, scan the result into our tag composite object
	            err = results.Scan(&msg.Uid)
	            if err != nil {
	                log.Printf(err.Error())
	                return err
	            }
	
		    ch <- msg
	
        	}
	}

/*
	i := 1

	err := filepath.Walk(mbox.Path, func(path string, info os.FileInfo, err error) error {
	  if !info.IsDir() {
		seqNum := uint32(i + 1)

		log.Printf("ListMessage id %d", seqNum)

		var id uint32
		id = seqNum

		if !seqSet.Contains(id) {
			return nil
		}

		msg := &imap.Message{
			Uid: id,
			//Path: path,
		}

		ch <- msg

		i++
	  }
	  return nil
	})

	return err
	*/
	return nil
}

func (mbox *Mailbox) SearchMessages(uid bool, criteria *imap.SearchCriteria) ([]uint32, error) {
	var ids []uint32
	var id uint32
	log.Printf("SearchMessages %s %s", uid, criteria)

        results, err := db.Query("SELECT message FROM mappings WHERE mailbox = ?", mbox.Id)
        if err != nil {
            log.Printf(err.Error())
            return nil, err
        }

        for results.Next() {
            // for each row, scan the result into our tag composite object
            err = results.Scan(&id)
            if err != nil {
                log.Printf(err.Error())
                return ids, err
            }

            ids = append(ids, id)
        }
/*
	for i, msg := range mbox.Messages {
		seqNum := uint32(i + 1)

		ok, err := msg.Match(seqNum, criteria)
		if err != nil || !ok {
			continue
		}

		var id uint32
		if uid {
			id = msg.Uid
		} else {
			id = seqNum
		}
		ids = append(ids, id)
	}
*/
//      log.Printf("%s",ids)
	return ids, nil
}

func (mbox *Mailbox) IndexFolder() error{

        log.Printf("index folder %s", mbox.Path)
	err := filepath.Walk(mbox.Path, func(path string, info os.FileInfo, err error) error {
	  if err != nil {
		log.Printf("file %s", err.Error())
		return err
	  }
	  if !info.IsDir() {
		log.Printf("file %s", info.Name())
		content, err := ioutil.ReadFile(path)
		if err != nil {
			return err
		}
		mbox.IndexMessage(content, info.Name())

	  }
	  return nil
	})
	return err
}

func (mbox *Mailbox) IndexMessage(body []byte, path string) error {

	var headers string //TODO
	var id int64

	insert, err := db.Exec("INSERT INTO messages (date, flags, size, headers, path) VALUES (NOW(), '', ?, ?, ?)", len(body), headers, path)
	// if there is an error inserting, handle it
	if err != nil {
		log.Printf(err.Error())
		return err
	} else {
		id, err = insert.LastInsertId()
	        if err != nil {
			log.Printf(err.Error())
			return err
		}
	}

	insert2, err := db.Query("INSERT INTO mappings (mailbox, message) VALUES (?, ?)", mbox.Id, id)
	// if there is an error inserting, handle it
	if err != nil {
		panic(err.Error())
		return err
	}
	// be careful deferring Queries if you are using transactions
	defer insert2.Close()

	return nil
}

func (mbox *Mailbox) CreateMessage(flags []string, date time.Time, body imap.Literal) error {
	if date.IsZero() {
		date = time.Now()
	}

	b, err := ioutil.ReadAll(body)
	if err != nil {
		return err
	}

	msg := &Message{
		Uid:   mbox.uidNext(),
		Date:  date,
		Size:  uint32(len(b)),
		Flags: flags,
		Content:  b,
	}

	log.Printf("new msg %s", msg)

	return nil
}

func (mbox *Mailbox) UpdateMessagesFlags(uid bool, seqset *imap.SeqSet, op imap.FlagsOp, flags []string) error {
	for i, msg := range mbox.Messages {
		var id uint32
		if uid {
			id = msg.Uid
		} else {
			id = uint32(i + 1)
		}
		if !seqset.Contains(id) {
			continue
		}

		msg.Flags = backendutil.UpdateFlags(msg.Flags, op, flags)
	}

	return nil
}

func (mbox *Mailbox) CopyMessages(uid bool, seqset *imap.SeqSet, destName string) error {
	_, err := mbox.user.GetMailbox(destName)
	if err != nil {
		return err
	}

	for i, msg := range mbox.Messages {
		var id uint32
		if uid {
			id = msg.Uid
		} else {
			id = uint32(i + 1)
		}
		if !seqset.Contains(id) {
			continue
		}

	//	msgCopy := *msg
	//	msgCopy.Uid = dest.uidNext()
	//	dest.Messages = append(dest.Messages, &msgCopy)
	}

	return nil
}

func (mbox *Mailbox) Expunge() error {
	for i := len(mbox.Messages) - 1; i >= 0; i-- {
		msg := mbox.Messages[i]

		deleted := false
		for _, flag := range msg.Flags {
			if flag == imap.DeletedFlag {
				deleted = true
				break
			}
		}

		if deleted {
			mbox.Messages = append(mbox.Messages[:i], mbox.Messages[i+1:]...)
		}
	}

	return nil
}
