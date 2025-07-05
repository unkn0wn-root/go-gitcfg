package main

import (
    "fmt"
    "github.com/unkn0wn-root/gitcfg"
)

func main() {
    // Load global Git configuration
    config, err := gitcfg.LoadLocal("/Users/david0/Git/myrepos/gitgo-cfg")
    if err != nil {
        panic(err)
    }

    // Get user information
    user, err := config.GetUser()
    if err != nil {
        panic(err)
    }

    fmt.Printf("User: %s <%s>\n", user.Name, user.Email)
    url, err := config.GetRemoteURL("origin")
    if err != nil {
        panic(err)
    }
    fmt.Printf("Remote URL: %s\n", url)
}
