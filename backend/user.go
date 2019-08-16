package memory

import (
	"errors"
	"fmt"
        "os"
        "path/filepath"
	"strings"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/backend"
)

type User struct {
	id	  int32
	username  string
	password  string
	path      string
//	mailboxes map[string]*Mailbox
}

func (u *User) Username() string {
	return u.username
}

func (u *User) ListMailboxesNative(subscribed bool) (mailboxes []*Mailbox, err error) {
	// Execute the query
	results, err := db.Query("SELECT id,name,subscribed FROM mailboxes WHERE user = ?", 1)
	defer results.Close()
	if err != nil {
	    DoLog(err.Error())
	    return mailboxes, err
	}

	for results.Next() {
	    mailbox := &Mailbox{
		    Path: u.path + "cur/",
		    user: u,
	    }
	    // for each row, scan the result into our tag composite object
	    err = results.Scan(&mailbox.Id, &mailbox.name, &mailbox.Subscribed)
	    if err != nil {
	        DoLog(err.Error())
	        return mailboxes, err
	    }

	    if subscribed && !mailbox.Subscribed {
		    continue
	    }

	    mailboxes = append(mailboxes, mailbox)
	}
	return mailboxes, err
}

func (u *User) ListMailboxes(subscribed bool) (mailboxes []backend.Mailbox, err error) {
	mboxes, err := u.ListMailboxesNative(false)
	if err != nil {
		DoLog(err.Error())
		return
	}

	for _,mbox := range mboxes {
		mailboxes = append(mailboxes, mbox)
	}
	return
}

func (u *User) GetMailboxNative(name string) (mailbox *Mailbox, err error) {
	mailboxes, err := u.ListMailboxesNative(false)
	if err != nil {
		DoLog(err.Error())
		return
	}

	for _,mbox := range mailboxes {
		if mbox.name == name {
			return mbox, nil
		}
	}

	return nil, errors.New("No such mailbox")
}

func (u *User) GetMailbox(name string) (mailbox backend.Mailbox, err error) {
	return u.GetMailboxNative(name)
}

func (u *User) CreateMailboxNative(name string, dynamic bool) (mailbox *Mailbox, err error) {
	if name == "reindex" {
		err := u.ReIndexMailbox()
		if err != nil {
			DoLog(err.Error())
		}
		return nil, err
	}

	mailbox, err = u.GetMailboxNative(name)
	if err == nil {
		return
	}

	_, err = db.Exec("INSERT INTO mailboxes (name, user, subscribed, dynamic) VALUES (?, ?, ?, ?)", name, u.id, 1, dynamic)
        // if there is an error inserting, handle it
        if err != nil {
		DoLog(err.Error())
                return
        }

	mailbox, err = u.GetMailboxNative(name)
        if err != nil {
		DoLog(err.Error())
        }
	return
}

func (u *User) CreateMailbox(name string) error {
	_, err := u.CreateMailboxNative(name, false)
        if err != nil {
		DoLog(err.Error())
        }
	return err
}

func (u *User) DeleteMailbox(name string) error {
	if name == "INBOX" {
		return errors.New("Cannot delete INBOX")
	}
	mbox, err := u.GetMailboxNative(name)
	if err != nil {
		return errors.New("No such mailbox")
	}

	_, err = db.Exec("DELETE FROM mappings WHERE mailbox = ?", mbox.Id)
        if err != nil {
		DoLog(err.Error())
                return err
        }

	_, err = db.Exec("DELETE FROM mailboxes WHERE id = ?", mbox.Id)
        // if there is an error inserting, handle it
        if err != nil {
		DoLog(err.Error())
                return err
        }

	return err
}

func (u *User) RenameMailbox(existingName, newName string) error {
	mbox, err := u.GetMailboxNative(existingName)
	if err != nil {
		DoLog(err.Error())
		return err
	}

	_, err = u.GetMailboxNative(newName)
	if err == nil {
		return errors.New("Mailbox already exists")
	}

	if existingName == "INBOX" {
		return errors.New("Cannot rename INBOX")
	}

	_, err = db.Exec("UPDATE mailboxes SET name=? WHERE id = ?", newName, mbox.Id)
        if err != nil {
		DoLog(err.Error())
                return err
        }

	return nil
}

func (u *User) IndexNew() error{
	mpath := u.path + "new/"
	cpath := u.path + "cur/"

        DoLog("index new %s", mpath)

        err := filepath.Walk(mpath, func(path string, info os.FileInfo, err error) error {
          if err != nil {
		DoLog(err.Error())
                return err
          }
          if !info.IsDir() {
                DoLog("new mail %s", info.Name())

		newpath := cpath + info.Name()
		err = os.Rename(path, newpath)
		DoLog("rename %s -> %s",path, newpath)
		if err != nil {
			DoLog(err.Error())
			return err
		}

                err = u.IndexMessage(info.Name(), info.Size())
		if err != nil {
			DoLog(err.Error())
			return err
		}

		DoLog("success")

          }
          return nil
        })

        return err
}

func (u *User) ReIndexMailbox() error{
	mpath := u.path + "cur/"

        DoLog("index folder %s", mpath)

        _, err := db.Query("DELETE FROM mappings WHERE user = ?", u.id)
        if err != nil {
                DoLog(err.Error())
                return err
        }

        _, err = db.Query("DELETE FROM mailboxes WHERE dynamic = 1 AND user = ?", u.id)
        if err != nil {
                DoLog(err.Error())
                return err
        }

        _, err = db.Query("UPDATE messages SET indexed=0 WHERE user = ?", u.id)
        if err != nil {
                DoLog(err.Error())
                return err
        }

        err = filepath.Walk(mpath, func(path string, info os.FileInfo, err error) error {
          if err != nil {
                DoLog("file %s", err.Error())
                return err
          }
          if !info.IsDir() {
                DoLog("file %s", info.Name())
                u.IndexMessage(info.Name(), info.Size())

          }
          return nil
        })

	_, err = db.Query("UPDATE users SET reindex=0 WHERE id = ?", u.id)
        if err != nil {
                DoLog(err.Error())
                return err
        }

        return err
}

func (u *User) IndexMessage(path string, length int64) error {

        var headers string //TODO
        var id int64

	path = strings.Split(path, ":")[0]

        insert, err := db.Exec("REPLACE INTO messages (date, user, indexed, flags, size, headers, path) VALUES (NOW(), ?, 1, '', ?, ?, ?)", u.id, length, headers, path)
        if err != nil {
                DoLog(err.Error())
                return err
        } else {
                id, err = insert.LastInsertId()
                if err != nil {
                	DoLog(err.Error())
                        return err
                }
        }

	m, err := GetMessage(uint32(id))
        if err != nil {
                DoLog(err.Error())
                return err
	}

	mp, err := m.Parse()
        if err != nil {
                DoLog(err.Error())
                return err
	}

	orig_to := mp.Header.Get("X-Original-To")

	all, err := u.CreateMailboxNative("ALL", true)
	if err != nil {
                DoLog(err.Error())
		return err
	}

	_, err = u.CreateMailboxNative(orig_to, true)
	if err != nil {
                DoLog(err.Error())
		return err
	}

	seqSet, _ := imap.ParseSeqSet(fmt.Sprintf("%d:%d", m.Uid, m.Uid))

	all.CopyMessages(true, seqSet, "ALL")
	all.CopyMessages(true, seqSet, orig_to)

        return nil
}

func (u *User) Logout() error {
	return nil
}
