package main

import (
	"fmt"
	"golang.org/x/crypto/bcrypt"
)

func main() {
	password := "admin"
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		panic(err)
	}
	fmt.Println("Password:", password)
	fmt.Println("Bcrypt Hash:", string(hash))
}
