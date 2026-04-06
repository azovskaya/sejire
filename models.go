package main

// Tree — для будущего использования (владелец, участники).
// API использует структуру Person из main.go с полями ID, TargetID, FullName и т.д.
type Tree struct {
	OwnerEmail string   `json:"owner_email"`
	Members    []Person `json:"members"`
}
