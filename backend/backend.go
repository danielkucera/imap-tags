// A memory backend.
package memory

import (
	"errors"
	"log"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/backend"

	"database/sql"
	_ "github.com/go-sql-driver/mysql"
)

var db *sql.DB

type Backend struct {
}

func (be *Backend) Login(coninfo *imap.ConnInfo, username, password string) (backend.User, error) {
	var err error

	log.Printf("login %s from %s", username, coninfo.RemoteAddr)
	defer log.Printf("login %s from %s err %s", username, coninfo.RemoteAddr, err)

	reindex := false

	user := &User{
		username: username,
	}

        err = db.QueryRow("SELECT id, mailbox_path, reindex FROM users WHERE name = ? AND pass = SHA1(?)", username, password).Scan(&user.id, &user.path, &reindex)
	if err == sql.ErrNoRows {
	    return nil, errors.New("Bad username or password")
	}

        if err != nil {
            log.Printf(err.Error())
            return nil, err
        }

	if reindex {
	    user.ReIndexMailbox()
	}

	return user, nil
}

func New(db_string string) *Backend {

	var err error
	// Open up our database connection.
	db, err = sql.Open("mysql", db_string+"?parseTime=true")

	// if there is an error opening the connection, handle it
	if err != nil {
		log.Printf(err.Error())
	    panic(err.Error())
	}

	return &Backend{}
}
