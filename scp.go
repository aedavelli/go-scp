/*
This package provides simple SCP client for copying data recursively to remote server. It's built
on top of x/crypto/ssh

*/
package scp // import "github.com/aedavelli/go-scp"

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/kballard/go-shellquote"
	"golang.org/x/crypto/ssh"
)

type Client struct {
	SshClient    *ssh.Client
	PreseveTimes bool
	Quiet        bool
}

// Form send command based on client configuration
func (c *Client) getSendCommand(dst string) string {
	cmd := "scp -rt"

	if c.PreseveTimes {
		cmd += "p"
	}

	if c.Quiet {
		cmd += "q"
	}

	return fmt.Sprintf("%s %s", cmd, shellquote.Join(dst))
}

// Send the files dst directory on remote side. The paths can be regular files or directories.
func (c *Client) Send(dst string, paths ...string) error {
	// Create an SSH session
	session, err := c.SshClient.NewSession()
	if err != nil {
		return errors.New("Failed to create SSH session: " + err.Error())
	}
	defer session.Close()

	// Setup Input strem
	w, err := session.StdinPipe()
	if err != nil {
		return errors.New("Unable to get stdin: " + err.Error())
	}
	defer w.Close()

	// Setup Output strem
	r, err := session.StdoutPipe()
	if err != nil {
		return errors.New("Unable to get Stdout: " + err.Error())
	}

	fmt.Println(c.getSendCommand(dst))
	if err := session.Start(c.getSendCommand(dst)); err != nil {
		return errors.New("Failed to start: " + err.Error())
	}

	errors := make(chan error)

	go func() {
		errors <- session.Wait()
	}()

	for _, p := range paths {
		if err := c.walkAndSend(w, p); err != nil {
			return err
		}
	}
	w.Close()
	io.Copy(os.Stdout, r)
	<-errors

	return nil
}

// send regular file
func (c *Client) sendRegularFile(w io.Writer, path string, fi os.FileInfo) error {
	if c.PreseveTimes {
		_, err := fmt.Fprintf(w, "T%d 0 %d 0\n", fi.ModTime().Unix(), time.Now().Unix())
		if err != nil {
			return err
		}
	}
	_, err := fmt.Fprintf(w, "C%#o %d %s\n", fi.Mode().Perm(), fi.Size(), fi.Name())
	if err != nil {
		return errors.New("Copy failed: " + err.Error())
	}
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	io.Copy(w, f)
	fmt.Fprint(w, "\x00")
	if !c.Quiet {
		fmt.Println("Copied: ", path)
	}
	return nil
}

// Walk and Send directory
func (c *Client) walkAndSend(w io.Writer, src string) error {
	cleanedPath := filepath.Clean(src)

	fi, err := os.Stat(cleanedPath)
	if err != nil {
		return err
	}

	if fi.Mode().IsRegular() {
		if err = c.sendRegularFile(w, cleanedPath, fi); err != nil {
			return err
		}
	}

	// It is a directory need to walk and copy
	dirStack := strings.Split(cleanedPath, fmt.Sprintf("%c", os.PathSeparator))
	startStackLen := len(dirStack)
	dirStack = dirStack[:startStackLen-1]
	startStackLen--

	err = filepath.Walk(cleanedPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		tmpDirStack := strings.Split(path, fmt.Sprintf("%c", os.PathSeparator))
		i, di, ci := 0, 0, 0
		dl, cl := len(dirStack), len(tmpDirStack)

		if info.Mode().IsRegular() {
			tmpDirStack = tmpDirStack[:cl-1]
			cl--
		}

		for i = 0; i < dl && i < cl; i++ {
			if dirStack[i] != tmpDirStack[i] {
				break
			}
			di++
			ci++
		}

		for di < dl { // We need to pop
			fmt.Fprintf(w, "E\n")
			di++
		}

		for ci < cl { // We need to push
			if c.PreseveTimes {
				_, err := fmt.Fprintf(w, "T%d 0 %d 0\n", info.ModTime().Unix(), time.Now().Unix())
				if err != nil {
					return err
				}
			}
			fmt.Fprintf(w, "D%#o 0 %s\n", info.Mode().Perm(), tmpDirStack[ci])
			ci++
		}

		dirStack = tmpDirStack
		if info.Mode().IsRegular() {
			if err = c.sendRegularFile(w, path, info); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}

	dl := len(dirStack) - 1

	for dl >= startStackLen {
		fmt.Fprintf(w, "E\n")
		dl--
	}
	return nil
}

// Creates a new SCP client. Use this only with trusted servers, as the host key verification
// is bypassed. It enables preserve time stamps
func NewDumbClient(username, password, server string) (*Client, error) {
	client, err := ssh.Dial("tcp", server, &ssh.ClientConfig{
		User: username,
		Auth: []ssh.AuthMethod{
			ssh.Password(password),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	})

	if err != nil {
		return nil, err
	}

	return &Client{
		SshClient:    client,
		PreseveTimes: true,
	}, nil
}

// Creates a new SCP client form ssh.Client and preserve time stamps
func NewClient(c *ssh.Client, pt bool) *Client {
	return &Client{
		SshClient:    c,
		PreseveTimes: pt,
	}
}
