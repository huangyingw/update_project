package main

import (
    "fmt"
    "os"

    "projupdater/cmd"
)

func main() {
    if err := cmd.Execute(); err != nil {
        fmt.Println("Error:", err)
        os.Exit(1)
    }
}
