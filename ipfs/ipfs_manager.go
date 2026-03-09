package ipfs

import (
	"bufio"
	"context"
	"explorer-server/config"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// Rubix MainNet bootstrap nodes
var MainNetBootstrap = []string{
	"/ip4/161.35.169.251/tcp/4001/p2p/12D3KooWPhZEYEw4jG3kSRuwgMEHcVt7KMkm1ui2ddu4fgSgwvDq",
	"/ip4/103.127.158.120/tcp/4001/p2p/12D3KooWSQ94HRDzFf6W2rp7P8gzP6efZQHTaSU8uaQjskVBHiWP",
	"/ip4/172.104.191.191/tcp/4001/p2p/12D3KooWFudnWZY1v1m4YXCzDWZSbNt7nvf5F42uzM6vErZ4NwqJ",
}

// Rubix TestNet bootstrap nodes
var TestNetBootstrap = []string{
	"/ip4/103.209.145.177/tcp/4001/p2p/12D3KooWD8Rw7Fwo4n7QdXTCjbh6fua8dTqjXBvorNz3bu7d9xMc",
	"/ip4/98.70.52.158/tcp/4001/p2p/12D3KooWQyWFABF3CKFnzX85hf5ZwrT5zPsy4rWHdGPZ8bBpRVCK",
	"/ip4/20.244.16.143/tcp/4001/p2p/12D3KooWAydFDJeSW5qupmp3AjRxc82Dq1AnjfJT1zwy4hg2TuNn",
	"/ip4/40.81.232.217/tcp/4001/p2p/12D3KooWK6V21GQotbub3cfgb5qAK1uUoUGPexf3vsLqw6yBJfen",
}

// IPFSManager handles the lifecycle of the IPFS daemon process
type IPFSManager struct {
	ctx      context.Context
	cancel   context.CancelFunc
	cmd      *exec.Cmd
	ipfsPath string // resolved path to ipfs executable
}

// NewIPFSManager creates a new manager instance
func NewIPFSManager() *IPFSManager {
	ctx, cancel := context.WithCancel(context.Background())
	return &IPFSManager{
		ctx:    ctx,
		cancel: cancel,
	}
}

// findIPFSExecutable looks for the bundled ipfs executable in the ipfs_bin/ directory:
func findIPFSExecutable() (string, error) {
	// Determine the correct bundled binary name for the OS
	ipfsBinary := "ipfs-linux"
	if runtime.GOOS == "windows" {
		ipfsBinary = "ipfs-windows.exe"
	} else if runtime.GOOS == "darwin" {
		ipfsBinary = "ipfs-mac"
	}

	// 1. Check same directory as the running binary (Production/Compiled)
	exePath, err := os.Executable()
	if err == nil {
		exeDir := filepath.Dir(exePath)
		localIPFS := filepath.Join(exeDir, "ipfs", "ipfs_bin", ipfsBinary)
		if _, err := os.Stat(localIPFS); err == nil {
			log.Printf("Found bundled IPFS binary: %s", localIPFS)
			return localIPFS, nil
		}
	}

	// 2. Check current working directory (useful during development with `go run`)
	cwd, err := os.Getwd()
	if err == nil {
		cwdIPFS := filepath.Join(cwd, "ipfs", "ipfs_bin", ipfsBinary)
		if _, err := os.Stat(cwdIPFS); err == nil {
			log.Printf("Found bundled IPFS binary in dev directory: %s", cwdIPFS)
			return cwdIPFS, nil
		}
	}

	// 3. Fallback to system PATH (if they installed it globally themselves)
	pathIPFS, err := exec.LookPath("ipfs")
	if err == nil {
		log.Printf("Warning: Bundled IPFS not found. Falling back to system IPFS binary: %s", pathIPFS)
		return pathIPFS, nil
	}

	return "", fmt.Errorf("ipfs executable not found. Place %s in the ipfs/ipfs_bin/ folder, or install IPFS (Kubo) and add it to PATH", ipfsBinary)
}

// getIPFSRepoPath returns the path to the IPFS repository
func getIPFSRepoPath() (string, error) {
	if ipfsPath := os.Getenv("IPFS_PATH"); ipfsPath != "" {
		return ipfsPath, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user home dir: %w", err)
	}
	return filepath.Join(home, ".ipfs"), nil
}

// EnsureInitialized checks if IPFS is available, initialized, and configured for Rubix network
func (m *IPFSManager) EnsureInitialized(testNet bool, customSwarmKeyPath string) error {
	// 0. Force private network mode (same as Rubix node)
	os.Setenv("LIBP2P_FORCE_PNET", "1")
	log.Println("LIBP2P_FORCE_PNET=1 (private network mode enforced)")

	// 1. Find ipfs executable
	ipfsPath, err := findIPFSExecutable()
	if err != nil {
		return err
	}
	m.ipfsPath = ipfsPath

	// 2. Check if .ipfs repository exists
	ipfsRepo, err := getIPFSRepoPath()
	if err != nil {
		return err
	}

	if _, err := os.Stat(ipfsRepo); os.IsNotExist(err) {
		log.Println("IPFS repository not found. Initializing...")
		cmd := exec.Command(m.ipfsPath, "init")
		output, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("failed to initialize IPFS: %w\nOutput: %s", err, string(output))
		}
		log.Println("IPFS initialized successfully")
	}

	// 3. Write the appropriate swarm key
	swarmKeyDest := filepath.Join(ipfsRepo, "swarm.key")
	var swarmKeyData []byte

	if customSwarmKeyPath != "" {
		// Custom swarm key provided via -swarmkey flag
		swarmKeyData, err = os.ReadFile(customSwarmKeyPath)
		if err != nil {
			return fmt.Errorf("failed to read custom swarm key from %s: %w", customSwarmKeyPath, err)
		}
		log.Printf("Using custom swarm key (%s)", customSwarmKeyPath)
	} else if testNet {
		swarmKeyData = config.TestNetSwarmKey
		log.Println("Using TestNet swarm key (testswarm.key)")
	} else {
		swarmKeyData = config.MainNetSwarmKey
		log.Println("Using MainNet swarm key (swarm.key)")
	}
	if err := os.WriteFile(swarmKeyDest, swarmKeyData, 0600); err != nil {
		return fmt.Errorf("failed to write swarm key: %w", err)
	}

	// 4. Remove all default public bootstrap nodes
	log.Println("Removing default IPFS bootstrap nodes...")
	exec.Command(m.ipfsPath, "bootstrap", "rm", "--all").Run()

	// 5. Add the Rubix bootstrap nodes (Skip if using a custom completely private network)
	var bootstrapNodes []string
	var networkName string

	if customSwarmKeyPath != "" {
		networkName = "Custom"
		// In a custom network, users must add their own bootstrap nodes manually or via config
		// We leave the bootstrap list completely empty so the daemon starts up cleanly.
	} else if testNet {
		bootstrapNodes = TestNetBootstrap
		networkName = "TestNet"
	} else {
		bootstrapNodes = MainNetBootstrap
		networkName = "MainNet"
	}

	if len(bootstrapNodes) > 0 {
		log.Printf("Configuring %s bootstrap nodes...", networkName)
	} else {
		log.Printf("Custom Network detected: Skipping default bootstrap nodes")
	}

	for _, node := range bootstrapNodes {
		cmd := exec.Command(m.ipfsPath, "bootstrap", "add", node)
		if output, err := cmd.CombinedOutput(); err != nil {
			log.Printf("Warning: Failed to add bootstrap node %s: %v\n%s", node, err, string(output))
		}
	}
	if len(bootstrapNodes) > 0 {
		log.Printf("Added %d %s bootstrap nodes", len(bootstrapNodes), networkName)
	}

	// 6. Enable Experimental.Libp2pStreamMounting (required by Rubix)
	cfgCmd := exec.Command(m.ipfsPath, "config", "--json", "Experimental.Libp2pStreamMounting", "true")
	if output, err := cfgCmd.CombinedOutput(); err != nil {
		log.Printf("Warning: Failed to enable Libp2pStreamMounting: %v\n%s", err, string(output))
	} else {
		log.Println("Experimental.Libp2pStreamMounting enabled")
	}

	// 7. Set IPFS API address to port 5001 (consistent with our PubSub client)
	apiCmd := exec.Command(m.ipfsPath, "config", "Addresses.API", "/ip4/127.0.0.1/tcp/5001")
	if output, err := apiCmd.CombinedOutput(); err != nil {
		log.Printf("Warning: Failed to set API address: %v\n%s", err, string(output))
	} else {
		log.Println("IPFS API address set to /ip4/127.0.0.1/tcp/5001")
	}

	// 9. Allow API access from the local client module
	exec.Command(m.ipfsPath, "config", "--json", "API.HTTPHeaders.Access-Control-Allow-Origin", `["*"]`).Run()
	exec.Command(m.ipfsPath, "config", "--json", "API.HTTPHeaders.Access-Control-Allow-Methods", `["PUT", "POST", "GET"]`).Run()

	return nil
}

// StartDaemon launches the ipfs daemon as a child process with pubsub enabled
func (m *IPFSManager) StartDaemon() error {
	m.cmd = exec.CommandContext(m.ctx, m.ipfsPath, "daemon", "--enable-pubsub-experiment")

	stdout, err := m.cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := m.cmd.StderrPipe()
	if err != nil {
		return err
	}

	log.Println("Starting local IPFS daemon with PubSub enabled...")

	if err := m.cmd.Start(); err != nil {
		return fmt.Errorf("failed to start ipfs daemon: %w", err)
	}

	daemonReady := make(chan bool, 1)

	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			line := scanner.Text()
			if strings.Contains(line, "Daemon is ready") {
				log.Printf("[IPFS] %s", line)
				daemonReady <- true
			} else if strings.Contains(line, "Swarm listening on") || strings.Contains(line, "API server listening") {
				log.Printf("[IPFS] %s", line)
			}
		}
	}()

	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			errText := scanner.Text()
			if strings.Contains(errText, "Private networking (swarm.key / LIBP2P_FORCE_PNET) does not work with public HTTP IPNIs") {
				continue
			}
			log.Printf("[IPFS ERROR] %s", errText)
		}
	}()

	select {
	case <-daemonReady:
		log.Println("IPFS daemon is ready")
	case <-time.After(30 * time.Second):
		log.Println("Warning: Timed out waiting for IPFS daemon ready signal, proceeding anyway...")
	}

	return nil
}

// Stop sends the kill signal to the child process
func (m *IPFSManager) Stop() {
	if m.cancel != nil {
		log.Println("Stopping local IPFS daemon...")
		m.cancel()
	}
	if m.cmd != nil && m.cmd.Process != nil {
		done := make(chan error, 1)
		go func() {
			done <- m.cmd.Wait()
		}()

		select {
		case <-time.After(5 * time.Second):
			log.Println("Warning: IPFS daemon did not stop gracefully, forcing kill...")
			m.cmd.Process.Kill()
		case <-done:
			log.Println("IPFS daemon stopped")
		}
	}
}
