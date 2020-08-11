package server

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/fatih/structs"
	"github.com/gogo/googleapis/google/rpc"
	"github.com/gogo/protobuf/protoc-gen-gogo/generator"
	"github.com/gogo/protobuf/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"

	"github.com/a-berahman/bt-api-user/models"
	pbExample "github.com/a-berahman/bt-api-user/proto"
)

// Globally access of mongoDb collections
// var collection *mongo.Collection

type Backend struct {
	mu      *sync.RWMutex
	users   []*pbExample.User
	session *mgo.Session
}

var _ pbExample.UserServiceServer = (*Backend)(nil)

func New(s *mgo.Session) *Backend {
	return &Backend{
		mu:      &sync.RWMutex{},
		session: s,
	}
}
func addUserException(b *Backend, user *pbExample.User) (*types.Empty, error) {
	// if len(b.users) == 0 && user.GetRole() != pbExample.Role_ADMIN {
	// 	st := status.New(codes.InvalidArgument, "First user created must be an admin")
	// 	detSt, err := st.WithDetails(&rpc.BadRequest{
	// 		FieldViolations: []*rpc.BadRequest_FieldViolation{
	// 			{
	// 				Field:       "role",
	// 				Description: "The first user created must have the role of an ADMIN",
	// 			},
	// 		},
	// 	})
	// 	if err == nil {
	// 		return nil, detSt.Err()
	// 	}
	// 	return nil, st.Err()
	// }
	// if user.GetFirstName() == "" {
	// 	st := status.New(codes.InvalidArgument, "First Name is required")
	// 	detSt, err := st.WithDetails(&rpc.BadRequest{
	// 		FieldViolations: []*rpc.BadRequest_FieldViolation{
	// 			{
	// 				Field:       "first_name",
	// 				Description: "First Name should be inserted",
	// 			},
	// 		},
	// 	})
	// 	if err == nil {
	// 		return nil, detSt.Err()
	// 	}
	// 	return nil, st.Err()
	// }

	// if user.GetLastName() == "" {
	// 	st := status.New(codes.InvalidArgument, "Last Name is required")
	// 	detSt, err := st.WithDetails(&rpc.BadRequest{
	// 		FieldViolations: []*rpc.BadRequest_FieldViolation{
	// 			{
	// 				Field:       "last_name",
	// 				Description: "Last Name should be inserted",
	// 			},
	// 		},
	// 	})
	// 	if err == nil {
	// 		return nil, detSt.Err()
	// 	}
	// 	return nil, st.Err()
	// }

	// if user.GetEmail() == "" {
	// 	st := status.New(codes.InvalidArgument, "Email is required")
	// 	detSt, err := st.WithDetails(&rpc.BadRequest{
	// 		FieldViolations: []*rpc.BadRequest_FieldViolation{
	// 			{
	// 				Field:       "Email",
	// 				Description: "Email should be inserted",
	// 			},
	// 		},
	// 	})
	// 	if err == nil {
	// 		return nil, detSt.Err()
	// 	}
	// 	return nil, st.Err()
	// }

	// if user.GetPassword() == "" {
	// 	st := status.New(codes.InvalidArgument, "Password is required")
	// 	detSt, err := st.WithDetails(&rpc.BadRequest{
	// 		FieldViolations: []*rpc.BadRequest_FieldViolation{
	// 			{
	// 				Field:       "password",
	// 				Description: "Password should be inserted",
	// 			},
	// 		},
	// 	})
	// 	if err == nil {
	// 		return nil, detSt.Err()
	// 	}
	// 	return nil, st.Err()
	// }

	// Check user ID doesn't already exist
	for _, u := range b.users {
		if u.GetID() == user.GetID() {
			return nil, status.Error(codes.FailedPrecondition, "user already exists")
		}
	}
	return nil, nil
}
func (b *Backend) AddUser(ctx context.Context, user *pbExample.User) (*pbExample.UserID, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if isExist(b, user.GetEmail()) == true {
		return &pbExample.UserID{}, status.Errorf(
			codes.Internal,
			fmt.Sprintln("User already is exist"),
		)
	}

	if _, err := addUserException(b, user); err != nil {
		return nil, err
	}

	if user.GetCreateDate() == nil {
		now := time.Now()
		user.CreateDate = &now
	}

	// b.users = append(b.users, user)
	userId := bson.NewObjectId()
	data := models.User{
		ID:        userId,
		FirstName: user.GetFirstName(),
		LastName:  user.GetLastName(),
		Password:  user.GetPassword(),
		Email:     user.GetEmail(),
	}

	err := b.session.DB("bt-temp").C("users").Insert(data)
	if err != nil {
		// new(types.Empty)
		return &pbExample.UserID{}, status.Errorf(
			codes.Internal,
			fmt.Sprintf("Internal error: %v", err),
		)
	}

	return &pbExample.UserID{Id: userId.Hex()}, nil
}

func (b *Backend) ListUsers(req *pbExample.ListUsersRequest, srv pbExample.UserService_ListUsersServer) error {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if len(b.users) == 0 {
		st := status.New(codes.FailedPrecondition, "No users have been created")
		detSt, err := st.WithDetails(&rpc.PreconditionFailure{
			Violations: []*rpc.PreconditionFailure_Violation{
				{
					Type:        "USER",
					Subject:     "no users created",
					Description: "No users have been created",
				},
			},
		})
		if err == nil {
			return detSt.Err()
		}
		return st.Err()
	}

	for _, user := range b.users {
		switch {
		case req.GetCreatedSince() != nil && user.GetCreateDate().Before(*req.GetCreatedSince()):
			continue
		case req.GetOlderThan() != nil && time.Since(*user.GetCreateDate()) <= *req.GetOlderThan():
			continue
		}
		err := srv.Send(user)
		if err != nil {
			return err
		}
	}

	return nil
}

func (b *Backend) ListUsersByRole(req *pbExample.UserRole, srv pbExample.UserService_ListUsersByRoleServer) error {
	b.mu.RLock()
	defer b.mu.RUnlock()

	for _, user := range b.users {
		if user.GetRole() == req.GetRole() {
			err := srv.Send(user)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (b *Backend) UpdateUser(ctx context.Context, req *pbExample.UpdateUserRequest) (*pbExample.User, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	var user *pbExample.User
	for _, u := range b.users {
		if u.GetID() == req.GetUser().GetID() {
			user = u
		}
	}

	if user == nil {
		return nil, status.Error(codes.NotFound, "user was not found")
	}

	st := structs.New(user)
	for _, path := range req.GetUpdateMask().GetPaths() {
		if path == "id" {
			return nil, status.Error(codes.InvalidArgument, "cannot update id field")
		}
		// This doesn't translate properly if a CustomName setting is used,
		// but none of the fields except ID has that set, so NO WORRIES.
		fname := generator.CamelCase(path)
		field, ok := st.FieldOk(fname)
		if !ok {
			st := status.New(codes.InvalidArgument, "invalid field specified")
			st, err := st.WithDetails(&rpc.BadRequest{
				FieldViolations: []*rpc.BadRequest_FieldViolation{{
					Field:       "update_mask",
					Description: fmt.Sprintf("The user message type does not have a field called %q", path),
				}},
			})
			if err != nil {
				panic(err)
			}
			return nil, st.Err()
		}

		in := structs.New(req.GetUser())
		err := field.Set(in.Field(fname).Value())
		if err != nil {
			return nil, err
		}
	}

	return user, nil
}

func isExist(b *Backend, email string) bool {

	count, err := b.session.DB("bt-temp").C("users").Find(email).Count()
	if err != nil {
		log.Fatal("are ", err)

	}
	log.Fatal("are ", count)
	if count != 0 {
		return true
	}
	return false

}
