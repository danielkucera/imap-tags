package memory

import (
	"errors"
	"log"

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

func (u *User) Logout() error {
	return nil
}
