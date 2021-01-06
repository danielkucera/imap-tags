package memory

import (
	"errors"
	"fmt"
	"io/ioutil"
	"strings"
	"time"

	"github.com/emersion/go-imap"
)

var Delimiter = "/"

type Mailbox struct {
	Subscribed bool

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

func (mbox *Mailbox) uidMax() uint32 {
	var uid uint32
	err := db.QueryRow("SELECT IFNULL(MAX(id), 0)+1 FROM messages WHERE user = ?", mbox.user.id).Scan(&uid)
	if err != nil {
		DoLog(err.Error())
		return 0
	}

	uid++
	DoLog("%d", uid)
	return uid
}

func (mbox *Mailbox) uidNext() uint32 {
	var uid uint32
	err := db.QueryRow("SELECT IFNULL(MAX(message), 0)+1 FROM mappings WHERE mailbox = ?", mbox.Id).Scan(&uid)
	if err != nil {
		DoLog(err.Error())
		return 0
	}

	uid++
	DoLog("%d", uid)
	return uid
}

func (mbox *Mailbox) flags() []string {
	flagsMap := make(map[string]bool)
	/*
		for _, msg := range mbox.Messages {
			for _, f := range msg.Flags {
				if !flagsMap[f] {
					flagsMap[f] = true
				}
			}
		}
	*/
	var flags []string
	for f := range flagsMap {
		flags = append(flags, f)
	}
	return flags
}

func (mbox *Mailbox) messageCount() uint32 {
	var count uint32
	err := db.QueryRow("SELECT count(*) FROM mappings WHERE mailbox = ?", mbox.Id).Scan(&count)
	if err != nil {
		DoLog(err.Error())
		return 0
	}

	return count
}

func (mbox *Mailbox) Status(items []imap.StatusItem) (*imap.MailboxStatus, error) {
	mbox.user.IndexNew()
	status := imap.NewMailboxStatus(mbox.name, items)
	status.Flags = mbox.flags()
	status.PermanentFlags = []string{"\\*"}

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

func (mbox *Mailbox) seqSetToList(seqSet *imap.SeqSet) []uint32 {
	seqList := make([]uint32, 0)
	maxuid := mbox.uidMax()
	var muid uint32
	for muid = 1; muid < maxuid; muid++ {
		if seqSet.Contains(muid) {
			seqList = append(seqList, muid)
		}
	}
	return seqList
}

func (mbox *Mailbox) ListMessages(uid bool, seqSet *imap.SeqSet, items []imap.FetchItem, ch chan<- *imap.Message) error {
	DoLog("ListMessages %s %s %s", uid, seqSet.String(), items)
	defer close(ch)

	if uid {

		//TODO: sanitize seqSet
		seqList := mbox.seqSetToList(seqSet)
		for _, muid := range seqList {
			DoLog("Listing %d", muid)
			msg, err := GetMessage(muid)
			if err != nil {
				DoLog(err.Error())
				continue
			}

			m, err := msg.Fetch(muid, items)
			if err != nil {
				DoLog(err.Error())
				continue
			}

			DoLog("Listed %d", muid)
			ch <- m
		}
	} else {
		return errors.New("Not implemented")
	}

	DoLog("ListMessages finished")
	return nil
}

func (mbox *Mailbox) SearchMessages(uid bool, c *imap.SearchCriteria) ([]uint32, error) {
	var ids []uint32
	var id uint32

	DoLog("SearchMessages uid=%v criteria=%v", uid, c)

	query := "SELECT message FROM mappings LEFT JOIN messages ON mappings.message = messages.id WHERE mappings.mailbox = ? "

	if c.Uid != nil {
		query += fmt.Sprintf("AND mappings.message = %d", c.Uid)
	}

	results, err := db.Query(query, mbox.Id)
	defer results.Close()
	if err != nil {
		DoLog(err.Error())
		return nil, err
	}

	for results.Next() {
		// for each row, scan the result into our tag composite object
		err = results.Scan(&id)
		if err != nil {
			DoLog(err.Error())
			return ids, err
		}
		msg, err := GetMessage(id)
		if err != nil {
			DoLog(err.Error())
			return ids, err
		}
		/*
			    TODO: do matching
			    https://github.com/emersion/go-imap/blob/5a03a09eba6d2942e2903c4abd6435155d0b996b/backend/backendutil/search.go#L71
			    https://github.com/emersion/go-imap/blob/5a03a09eba6d2942e2903c4abd6435155d0b996b/search.go#L90

			    ok, err := msg.Match(0, criteria)
			    if err != nil || !ok {
				continue
			    }
		*/

		var id uint32
		if uid {
			id = msg.Uid
		} else {
			return nil, errors.New("Not implemented")
		}

		ids = append(ids, id)
	}
	return ids, nil
}

func (mbox *Mailbox) CreateMessage(flags []string, date time.Time, body imap.Literal) error {
	if date.IsZero() {
		date = time.Now()
	}

	b, err := ioutil.ReadAll(body)
	if err != nil {
		DoLog(err.Error())
		return err
	}

	msg := &Message{
		Uid:     mbox.uidNext(),
		Date:    date,
		Size:    uint32(len(b)),
		Flags:   flags,
		Content: b,
	}

	DoLog("new msg %s", msg)

	return nil
}

func (mbox *Mailbox) UpdateMessagesFlags(uid bool, seqset *imap.SeqSet, op imap.FlagsOp, flags []string) error {
	DoLog("updating flags: %v %v", seqset, flags)
	messages := make(chan *imap.Message)
	go func() {
		for {
			msg, more := <-messages
			if more {
				newFlagsList := make([]string, 0)
				flagSet := make(map[string]bool, 0)
				for _, setflag := range msg.Flags {
					flagSet[setflag] = true
				}
				for _, flag := range flags {
					if _, ok := flagSet[flag]; ok {
						delete(flagSet, flag)
					} else {
						flagSet[flag] = true
					}
				}
				for key := range flagSet {
					if key == "\\Deleted" {
						_, err := db.Exec("DELETE FROM `mappings` WHERE message = ? AND mailbox = ?", msg.Uid, mbox.Id)
						if err != nil {
							DoLog("mesage delete failed: %s", err)
						}
					} else {
						newFlagsList = append(newFlagsList, key)
					}
				}
				newflags := strings.Join(newFlagsList, ",")
				DoLog("updating flag for %v, %d, to %s", msg, msg.Uid, newflags)
				_, err := db.Exec("UPDATE `messages` SET flags=? WHERE id = ?", newflags, msg.Uid)
				if err != nil {
					DoLog("flag update failed: %s", err)
				} else {
					DoLog("flag update ok")
				}
			} else {
				return
			}
		}
	}()
	items := []imap.FetchItem{imap.FetchUid, imap.FetchFlags}
	err := mbox.ListMessages(uid, seqset, items, messages)
	if err != nil {
		return err
	}
	DoLog("updated flags: %v %v", seqset, flags)

	return nil
}

func (mbox *Mailbox) CopyMessages(uid bool, seqSet *imap.SeqSet, destName string) error {
	dest, err := mbox.user.GetMailboxNative(destName)
	if err != nil {
		DoLog(err.Error())
		return err
	}

	if !uid {
		return errors.New("Not implemented")
	}

	DoLog("copy messages %d", seqSet)

	var muid uint32
	maxuid := mbox.uidMax()

	for muid = 1; muid < maxuid; muid++ {
		if !seqSet.Contains(muid) {
			continue
		}
		DoLog("copying muid %d", muid)

		m, err := GetMessage(muid)
		if err != nil {
			DoLog("get message by id %d %s", muid, err.Error())
			continue
		}

		_, err = db.Exec("INSERT INTO mappings (user, mailbox, message) VALUES (?, ?, ?)", mbox.user.id, dest.Id, m.Uid)
		if err != nil {
			DoLog(err.Error())
			return err
		}

		DoLog("copied muid %d", muid)
	}

	return nil
}

func (mbox *Mailbox) Expunge() error {
	//Not needed, we are deleting on flag set
	return nil
}
