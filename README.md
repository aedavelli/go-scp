# go-scp
Simple SCP Package with Recursive copy support

## SCP Protocol Information
[How SCP Protocol Works](/SCP.md)

## Example
```go
package main

import (
	"log"
	"os"

	"github.com/aedavelli/go-scp"
)

func main() {
	if len(os.Args) < 2 {
		log.Fatal("Usage: ", os.Args[0], " <path1> <path2> ...")
	}
	c, err := scp.NewDumbClient("username", "password", "server.com:22")

	if err != nil {
		log.Fatal(err)
	}

	c.Quiet = true
	err = c.Send("/tmp", os.Args[1:]...)
	if err != nil {
		log.Fatal(err)
	}
}
```
