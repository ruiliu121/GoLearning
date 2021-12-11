package main

import (
	"github.com/pkg/errors"
	"database/sql"
)

type User struct {
	ID   int64  `sql:"column:id"`
	Name string `sql:"column:name"`
}

func GetUserInfo(db *sql.DB)(*[]User, error) {
	rows, err := db.Query(`SELECT * FROM users`)
	defer rows.Close()
	if err != nil {
		return nil, errors.Wrap(err, "get userInfo failed")
	}
	var users []User
	for rows.Next() {
		var user User
		err := rows.Scan(&user.ID, &user.Name)
		if err != nil {
			if err == sql.ErrNoRows {
				return "", errors.Wrap(err, "data is nil")
			}
			return nil, errors.Wrap(err, "read from db err")
		}
		users = append(users, user)
	}

	return &users, nil
}
