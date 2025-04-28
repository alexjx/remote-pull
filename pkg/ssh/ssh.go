package ssh

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

type sshConfig struct {
	HostName     string
	User         string
	Port         string
	IdentityFile string
}

func parseSSHConfig(host string) (*sshConfig, error) {
	config := &sshConfig{}
	configFile := filepath.Join(os.Getenv("HOME"), ".ssh", "config")

	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		return config, nil
	}

	file, err := os.Open(configFile)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	currentHost := ""
	inMatchingHost := false

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}

		key := strings.ToLower(fields[0])
		value := strings.Join(fields[1:], " ")

		if key == "host" {
			currentHost = value
			inMatchingHost = currentHost == host || currentHost == "*"
			continue
		}

		if !inMatchingHost {
			continue
		}

		switch key {
		case "hostname":
			config.HostName = value
		case "user":
			config.User = value
		case "port":
			config.Port = value
		case "identityfile":
			config.IdentityFile = strings.Replace(value, "~", os.Getenv("HOME"), 1)
		}
	}

	return config, scanner.Err()
}

type Client struct {
	*ssh.Client
}

func NewClient(user, host string) (*Client, error) {
	// Parse SSH config for this host
	sshConfig, err := parseSSHConfig(host)
	if err != nil {
		return nil, fmt.Errorf("failed to parse SSH config: %v", err)
	}

	// Use config values when available
	effectiveHost := host
	if sshConfig.HostName != "" {
		effectiveHost = sshConfig.HostName
	}

	effectiveUser := user
	if sshConfig.User != "" {
		effectiveUser = sshConfig.User
	}

	port := "22"
	if sshConfig.Port != "" {
		port = sshConfig.Port
	}

	authMethods := []ssh.AuthMethod{}

	// Try SSH agent auth if available
	if sock := os.Getenv("SSH_AUTH_SOCK"); sock != "" {
		if conn, err := net.Dial("unix", sock); err == nil {
			authMethods = append(authMethods, ssh.PublicKeysCallback(agent.NewClient(conn).Signers))
		}
	}

	// Try public key auth from standard locations and config
	keyPaths := []string{
		filepath.Join(os.Getenv("HOME"), ".ssh", "id_rsa"),
		filepath.Join(os.Getenv("HOME"), ".ssh", "id_ecdsa"),
		filepath.Join(os.Getenv("HOME"), ".ssh", "id_ed25519"),
	}
	if sshConfig.IdentityFile != "" {
		keyPaths = append(keyPaths, sshConfig.IdentityFile)
	}

	for _, keyPath := range keyPaths {
		if key, err := os.ReadFile(keyPath); err == nil {
			if signer, err := ssh.ParsePrivateKey(key); err == nil {
				authMethods = append(authMethods, ssh.PublicKeys(signer))
			}
		}
	}

	// Fall back to password auth if no other methods worked
	authMethods = append(authMethods, ssh.Password(""))

	config := &ssh.ClientConfig{
		User:            effectiveUser,
		Auth:            authMethods,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	client, err := ssh.Dial("tcp", effectiveHost+":"+port, config)
	if err != nil {
		return nil, fmt.Errorf("failed to dial: %v", err)
	}

	return &Client{client}, nil
}

func RunCommand(cmd, user, host string) (string, error) {
	client, err := NewClient(user, host)
	if err != nil {
		return "", err
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return "", fmt.Errorf("failed to create session: %v", err)
	}
	defer session.Close()

	// Connect command's stdout/stderr directly to console
	session.Stdout = os.Stdout
	session.Stderr = os.Stderr

	err = session.Run(cmd)
	if err != nil {
		return "", fmt.Errorf("command failed: %v", err)
	}

	return "", nil
}

func TransferFile(src, dest, user, host string) error {
	client, err := NewClient(user, host)
	if err != nil {
		return err
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create session: %v", err)
	}
	defer session.Close()

	go func() {
		w, _ := session.StdinPipe()
		defer w.Close()
		f, _ := os.Open(src)
		defer f.Close()
		io.Copy(w, f)
	}()

	if err := session.Run(fmt.Sprintf("/usr/bin/scp -qt %s", dest)); err != nil {
		return fmt.Errorf("failed to transfer file: %v", err)
	}

	return nil
}

func CopyAndRun(src, command, user, host string) error {
	client, err := NewClient(user, host)
	if err != nil {
		return err
	}
	defer client.Close()

	// Create a session for file transfer
	transferSession, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create transfer session: %v", err)
	}
	defer transferSession.Close()

	// Transfer the file with progress
	transferDone := make(chan error)
	go func() {
		defer close(transferDone)

		w, err := transferSession.StdinPipe()
		if err != nil {
			transferDone <- err
			return
		}
		defer w.Close()

		fileInfo, err := os.Stat(src)
		if err != nil {
			transferDone <- err
			return
		}
		fmt.Fprintf(w, "C0644 %d %s\n", fileInfo.Size(), filepath.Base(src))

		f, err := os.Open(src)
		if err != nil {
			transferDone <- err
			return
		}
		defer f.Close()

		fileInfo, err = f.Stat()
		if err != nil {
			transferDone <- err
			return
		}
		totalBytes := fileInfo.Size()
		var copiedBytes int64
		buf := make([]byte, 32*1024) // 32KB buffer

		for {
			n, err := f.Read(buf)
			if n > 0 {
				if _, err := w.Write(buf[:n]); err != nil {
					transferDone <- err
					return
				}
				copiedBytes += int64(n)
				progress := float64(copiedBytes) / float64(totalBytes) * 100
				fmt.Printf("\rTransferring: %.2f%%", progress)
			}
			if err == io.EOF {
				break
			}
			if err != nil {
				transferDone <- err
				return
			}
		}
		fmt.Fprint(w, "\x00")
		fmt.Println() // New line after progress
	}()

	// Execute the SCP command to receive the file
	transferSession.Stdout = os.Stdout
	transferSession.Stderr = os.Stderr

	if err := transferSession.Run("/usr/bin/scp -qt /tmp"); err != nil {
		return fmt.Errorf("scp transfer failed: %v", err)
	}

	// Wait for transfer to complete
	if err := <-transferDone; err != nil {
		return fmt.Errorf("file copy failed: %v", err)
	}

	// Create a new session for executing the command
	commandSession, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create command session: %v", err)
	}
	defer commandSession.Close()

	// Set up output for the command
	commandSession.Stdout = os.Stdout
	commandSession.Stderr = os.Stderr

	// Execute the final command in the new session
	fmt.Printf("Running command on remote server: %s\n", command)
	if err := commandSession.Run(command); err != nil {
		return fmt.Errorf("command failed: %v", err)
	}
	return nil
}
