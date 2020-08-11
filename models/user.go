package models

import "gopkg.in/mgo.v2/bson"

type User struct {
	ID        bson.ObjectId `bson:"_id"`
	FirstName string        `bson:"first_name"`
	LastName  string        `bson:"last_name"`
	Password  string        `bson:"password"`
	Email     string        `bson:"email"`
}

// ID        bson.ObjectId `bson:"_id,omitempty"`
