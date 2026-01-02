package main

import (
	"fmt"
	"os"

	mailrucloud "github.com/mentatxx/go-mail-ru-client"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: go-mail-ru-client <email> <password>")
		os.Exit(1)
	}

	email := os.Args[1]
	password := os.Args[2]

	account := mailrucloud.NewAccount(email, password)
	err := account.Login()
	if err != nil {
		fmt.Printf("Login failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Login successful!")

	// Get disk usage
	diskUsage, err := account.GetDiskUsage()
	if err != nil {
		fmt.Printf("Failed to get disk usage: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Total: %d, Used: %d, Free: %d\n",
		diskUsage.Total.DefaultValue,
		diskUsage.Used.DefaultValue,
		diskUsage.Free.DefaultValue)
}
