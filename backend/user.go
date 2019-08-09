package memory

import (
	"errors"
	"log"
        "os"
        "path/filepath"
	"io/ioutil"

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
	if err != nil {
	    log.Printf(err.Error())
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
	        log.Printf(err.Error())
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
		return
	}

	for _,mbox := range mboxes {
		mailboxes = append(mailboxes, mbox)
	}
	return
}

func (u *User) GetMailbox(name string) (mailbox backend.Mailbox, err error) {
	mailboxes, err := u.ListMailboxesNative(false)
	if err != nil {
		return
	}

	for _,mbox := range mailboxes {
		if mbox.name == name {
			return mbox, nil
		}
	}

	return nil, errors.New("No such mailbox")
}

func (u *User) CreateMailbox(name string) error {
	return errors.New("Not implemented")
}

func (u *User) DeleteMailbox(name string) error {
	if name == "INBOX" {
		return errors.New("Cannot delete INBOX")
	}
	_, err := u.GetMailbox(name)
	if err != nil {
		return errors.New("No such mailbox")
	}

	return errors.New("Not implemented")
	return nil
}

func (u *User) RenameMailbox(existingName, newName string) error {
	_, err := u.GetMailbox(existingName)
	if err != nil {
		return err
	}

	mbox, err := u.GetMailbox(existingName)
	if mbox != nil {
		return errors.New("Mailbox already exists")
	}

	if existingName == "INBOX" {
		return errors.New("Cannot rename INBOX")
	}

	return errors.New("Not implemented")

	return nil
}

func (u *User) IndexNew() error{
	mpath := u.path + "new/"
	cpath := u.path + "cur/"

        log.Printf("index new %s", mpath)

        err := filepath.Walk(mpath, func(path string, info os.FileInfo, err error) error {
          if err != nil {
                log.Printf("new walk err %s", err.Error())
                return err
          }
          if !info.IsDir() {
                log.Printf("new mail %s", info.Name())
                content, err := ioutil.ReadFile(path)
                if err != nil {
			log.Fatal(err)
                        return err
                }
                err = u.IndexMessage(content, info.Name())
		if err != nil {
			log.Fatal(err)
			return err
		}

		newpath := cpath + info.Name()
		err = os.Rename(path, newpath)
		log.Printf("rename %s -> %s",path, newpath)
		if err != nil {
			log.Fatal(err)
			return err
		}
		log.Printf("success")

          }
          return nil
        })

        return err
}

func (u *User) ReIndexMailbox() error{
	mpath := u.path + "cur/"

        log.Printf("index folder %s", mpath)

        _, err := db.Query("DELETE FROM mappings WHERE user = ?", u.id)
        // if there is an error inserting, handle it
        if err != nil {
                panic(err.Error())
                return err
        }

        _, err = db.Query("DELETE FROM messages WHERE user = ?", u.id)
        // if there is an error inserting, handle it
        if err != nil {
                panic(err.Error())
                return err
        }

        err = filepath.Walk(mpath, func(path string, info os.FileInfo, err error) error {
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
                u.IndexMessage(content, info.Name())

          }
          return nil
        })

	_, err = db.Query("UPDATE users SET reindex=0 WHERE id = ?", u.id)
        // if there is an error inserting, handle it
        if err != nil {
                panic(err.Error())
                return err
        }

        return err
}

func (u *User) IndexMessage(body []byte, path string) error {

        var headers string //TODO
        var id int64

        insert, err := db.Exec("INSERT INTO messages (date, user, flags, size, headers, path) VALUES (NOW(), ?, '', ?, ?, ?)", u.id, len(body), headers, path)
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

        insert2, err := db.Query("INSERT INTO mappings (user, mailbox, message) VALUES (?, ?, ?)", u.id, 1, id) //TODO: which mailbox id?
        // if there is an error inserting, handle it
        if err != nil {
                panic(err.Error())
                return err
        }
        // be careful deferring Queries if you are using transactions
        defer insert2.Close()

        return nil
}

func (u *User) Logout() error {
	return nil
}
