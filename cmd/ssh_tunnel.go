package cmd

import (
	"io"
	"log"
	"net"
	"os"
	"time"

	"golang.org/x/crypto/ssh"
)

type TunnelConfig struct {
	SSHUser        string `json:"SSHUser"`
	SSHHost        string `json:"SSHHost"`
	SSHPort        string `json:"SSHPort"`
	RemoteHost     string `json:"RemoteHost"`
	RemotePort     string `json:"RemotePort"`
	LocalPort      string `json:"LocalPort"`
	PrivateKeyPath string `json:"PrivateKeyPath"`
}

// SSHClient creates a new SSH client
func SSHClient(config TunnelConfig) (*ssh.Client, error) {
	key, err := os.ReadFile(config.PrivateKeyPath)
	if err != nil {
		log.Fatalf("unable to read private key: %v", err)
	}

	signer, err := ssh.ParsePrivateKey(key)
	if err != nil {
		log.Fatalf("unable to parse private key: %v", err)
	}

	// Define the SSH client configuration
	sshConfig := &ssh.ClientConfig{
		User: config.SSHUser,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // Don't verify host key (not recommended for production)
		Timeout:         5 * time.Second,             // Connection timeout
	}

	// Connect to the SSH server
	client, err := ssh.Dial("tcp", config.SSHHost+":"+config.SSHPort, sshConfig)
	if err != nil {
		return nil, err
	}

	return client, nil
}

// ForwardTraffic forwards traffic from local to remote host
func ForwardTraffic(localListener net.Listener, client *ssh.Client, config TunnelConfig) {
	for {
		localConn, err := localListener.Accept() // Accept local connection
		if err != nil {
			log.Printf("Failed to accept local connection: %v", err)
			continue
		}

		// Open a connection to the remote host
		remoteConn, err := client.Dial("tcp", config.RemoteHost+":"+config.RemotePort)
		if err != nil {
			log.Printf("Failed to connect to remote host: %v", err)
			localConn.Close()
			continue
		}

		// Forward data between local and remote connections
		go func() {
			defer localConn.Close()
			defer remoteConn.Close()

			// Forward local to remote
			go io.Copy(remoteConn, localConn)
			// Forward remote to local
			io.Copy(localConn, remoteConn)
		}()
	}
}

// StartSSHTunnel initializes the SSH tunnel and forwards traffic
func StartSSHTunnel(config *TunnelConfig) error {
	// Create an SSH client
	client, err := SSHClient(*config)
	if err != nil {
		return err
	}
	defer client.Close()

	// Listen on the local port
	localListener, err := net.Listen("tcp", "localhost:"+config.LocalPort)
	if err != nil {
		return err
	}
	defer localListener.Close()

	log.Printf("SSH tunnel started on localhost:%s forwarding to %s:%s", config.LocalPort, config.RemoteHost, config.RemotePort)

	// Forward the traffic between local and remote
	ForwardTraffic(localListener, client, *config)

	return nil
}
