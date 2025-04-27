# Remote Pull Tool

A CLI tool to securely transfer Docker images from local to remote servers via SSH.

## Features

- Basic Docker image transfer using SSH
- Automatic image existence checking to avoid redundant transfers
- Simple progress tracking during transfer

## Installation

### From Source
1. Clone the repository:
```bash
git clone https://github.com/xinj/remote-pull.git
cd remote-pull
```

2. Build the tool:
```bash
go build -o remote-pull ./cmd/docker-transfer
```

3. Install to your PATH (optional):
```bash
sudo mv remote-pull /usr/local/bin/
```

### Using Go Install
```bash
go install github.com/xinj/remote-pull/cmd/docker-transfer@latest
```

## Usage

Basic syntax:
```bash
remote-pull IMAGE_NAME USER@HOST [OPTIONS]
```

### Examples

Basic transfer:
```bash
remote-pull nginx:latest user@example.com
```

## Technical Details

### Transfer Process
1. Local image export using `docker save`
2. Transfer via SSH using `docker load` on remote
3. Basic progress tracking

### Remote Image Checking
Before transferring, the tool will:
1. Check if the specified Docker image exists on the remote server
2. Skip transfer if image exists

## Requirements
- Docker installed on both local and remote machines
- SSH access to remote server
- Go 1.16+ for building from source
- Proper SSH key configuration

## Authentication
The tool uses SSH key-based authentication. Make sure:
1. You have password-less SSH access to the remote server
2. Your SSH key is properly configured

## Troubleshooting

### Common Issues
- **SSH Connection Failed**
  - Verify SSH connectivity: `ssh USER@HOST`
  - Check firewall settings
  - Ensure SSH service is running on remote

- **Docker Permission Denied**
  - Add user to docker group: `sudo usermod -aG docker $USER`
  - Restart Docker service after group changes

- **Image Not Found**
  - Verify image exists locally: `docker images`
  - Check image name spelling

## License
MIT
