package models

// User represents a user in the system.
type User struct {
	ID        string `json:"id"`
	Username  string `json:"username"`
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
	Email     string `json:"email"`
}

// Group represents a group in the system.
type Group struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// GroupMembersResponse represents a response with a list of group members.
type GroupMembersResponse struct {
	Members []User `json:"members"`
}

type UserMembershipRequest struct {
	GroupID string `json:"groupId"`
	UserID  string `json:"userId"`
}
