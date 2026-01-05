package main

import (
	"fmt"
	"github.com/zalando/go-keyring"
)

func main() {
	service := "PerSSH_Test"
	user := "testuser"
	pass := "secret123"

	fmt.Println("Setting password...")
	err := keyring.Set(service, user, pass)
	if err != nil {
		fmt.Printf("Error setting: %v\n", err)
		return
	}

	fmt.Println("Getting password...")
	got, err := keyring.Get(service, user)
	if err != nil {
		fmt.Printf("Error getting: %v\n", err)
		return
	}

	if got != pass {
		fmt.Printf("Mismatch! Expected %s, got %s\n", pass, got)
	} else {
		fmt.Println("Success! Password matches.")
	}

	fmt.Println("Deleting password...")
	keyring.Delete(service, user)
}
