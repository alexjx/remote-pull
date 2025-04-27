package transfer

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"remote-pull/pkg/ssh"
)

func TransferImage(imageName, remoteServer string) error {
	// Split remote server into user and host
	parts := strings.Split(remoteServer, "@")
	if len(parts) != 2 {
		return fmt.Errorf("invalid remote server format, expected user@host")
	}
	user := parts[0]
	host := parts[1]

	// Check if image exists on remote
	fmt.Printf("[CHECKING] Verifying if %s exists on %s...\n", imageName, remoteServer)
	exists, err := checkRemoteImage(imageName, user, host)
	if err != nil {
		return fmt.Errorf("error checking remote image: %v", err)
	}

	if exists {
		fmt.Printf("[SKIPPING] Image %s already exists on %s - no transfer needed\n", imageName, remoteServer)
		return nil
	}
	fmt.Printf("[PROCEEDING] Image %s not found on %s - proceeding with transfer\n", imageName, remoteServer)

	// Pull image locally if needed
	if err := pullLocalImage(imageName); err != nil {
		return fmt.Errorf("error pulling local image: %v", err)
	}

	// Transfer image to remote
	if err := transferImage(imageName, user, host); err != nil {
		return fmt.Errorf("error transferring image: %v", err)
	}

	return nil
}

func checkRemoteImage(imageName, user, host string) (bool, error) {
	cmd := fmt.Sprintf("docker images -q %s", imageName)
	output, err := ssh.RunCommand(cmd, user, host)
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(output) != "", nil
}

func pullLocalImage(imageName string) error {
	cmd := exec.Command("docker", "pull", imageName)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func transferImage(imageName, user, host string) error {
	fmt.Printf("[CONNECTING] Establishing connection to '%s@%s' ...\n", user, host)

	// Create temp file for image tar
	tmpFile := fmt.Sprintf("/tmp/%s.tar", strings.ReplaceAll(imageName, "/", "_"))
	fmt.Printf("[PREPARING] Creating temporary archive at %s\n", tmpFile)

	// Save local image to tar file
	fmt.Printf("[SAVING] Exporting Docker image %q to archive\n", imageName)
	saveCmd := exec.Command("docker", "save", "-o", tmpFile, imageName)
	saveCmd.Stdout = os.Stdout
	saveCmd.Stderr = os.Stderr
	if err := saveCmd.Run(); err != nil {
		return fmt.Errorf("[ERROR] Failed to save image: %v", err)
	}
	defer func() {
		fmt.Printf("[CLEANUP] Removing temporary archive %s\n", tmpFile)
		cmd := exec.Command("rm", "-f", tmpFile)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Run()
	}()

	// Get file size for progress calculation
	fileInfo, err := os.Stat(tmpFile)
	if err != nil {
		return fmt.Errorf("[ERROR] Failed to get archive size: %v", err)
	}
	sizeMB := float64(fileInfo.Size()) / 1024 / 1024
	fmt.Printf("[STATUS] Archive size: %.2f MB\n", sizeMB)

	// Transfer tar file to remote host
	fmt.Printf("[TRANSFER] Starting transfer to %s (%.2f MB)\n", host, sizeMB)
	fmt.Println("[PROGRESS] Transfer in progress...")

	transferCmd := fmt.Sprintf("docker load -i %s", tmpFile)
	err = ssh.CopyAndRun(tmpFile, transferCmd, user, host)
	if err != nil {
		return fmt.Errorf("[ERROR] Transfer failed: %v", err)
	}

	fmt.Printf("[SUCCESS] Image %s successfully transferred and loaded on %s\n", imageName, host)
	return nil
}
