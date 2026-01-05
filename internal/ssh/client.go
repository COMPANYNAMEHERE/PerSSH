package ssh

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	"github.com/COMPANYNAMEHERE/PerSSH/internal/common"
)

// RemoteInterface defines the methods required for communicating with the agent.
type RemoteInterface interface {
	Connect() error
	Close()
	DeployAgent(localBinaryPath string) error
	StartAgent() error
	SendRequest(req common.Request) error
	GetStdout() io.Reader
}

type Client struct {
	Host     string
	User     string
	Port     int
	Auth     []ssh.AuthMethod
	Client   *ssh.Client
	Session  *ssh.Session
	Stdin    io.WriteCloser
	Stdout   io.Reader
	SFTP     *sftp.Client
}

func NewClient(host, user string, port int, password string, keyPath string) (*Client, error) {
	var authMethods []ssh.AuthMethod

	if keyPath != "" {
		key, err := os.ReadFile(keyPath)
		if err == nil {
			signer, err := ssh.ParsePrivateKey(key)
			if err == nil {
				authMethods = append(authMethods, ssh.PublicKeys(signer))
			}
		}
	}

	if password != "" {
		authMethods = append(authMethods, ssh.Password(password))
		// Also add KeyboardInteractive as a fallback for some servers
		authMethods = append(authMethods, ssh.KeyboardInteractive(func(user, instruction string, questions []string, echos []bool) (answers []string, err error) {
			answers = make([]string, len(questions))
			for i := range questions {
				answers[i] = password
			}
			return answers, nil
		}))
	}

	return &Client{
		Host: host,
		User: user,
		Port: port,
		Auth: authMethods,
	}, nil
}

func (c *Client) Connect() error {
	config := &ssh.ClientConfig{
		User:            c.User,
		Auth:            c.Auth,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // For prototype simplicity. Production should verify.
		Timeout:         10 * time.Second,
	}

	addr := fmt.Sprintf("%s:%d", c.Host, c.Port)
	client, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		return err
	}
	c.Client = client
	return nil
}

func (c *Client) Close() {
	if c.Session != nil {
		c.Session.Close()
	}
	if c.SFTP != nil {
		c.SFTP.Close()
	}
	if c.Client != nil {
		c.Client.Close()
	}
}

// DeployAgent uploads the perssh-server binary if needed.
func (c *Client) DeployAgent(localBinaryPath string) error {
	sftpClient, err := sftp.NewClient(c.Client)
	if err != nil {
		return err
	}
	c.SFTP = sftpClient

	remotePath := "./perssh-server"
	
	// Upload
	f, err := os.Open(localBinaryPath)
	if err != nil {
		return fmt.Errorf("cannot find local agent binary at %s: %w", localBinaryPath, err)
	}
	defer f.Close()

	dst, err := sftpClient.Create(remotePath)
	if err != nil {
		return err
	}
	defer dst.Close()

	if _, err := io.Copy(dst, f); err != nil {
		return err
	}

	// Chmod
	return sftpClient.Chmod(remotePath, 0755)
}

// StartAgent runs the agent and pipes IO.
func (c *Client) StartAgent() error {
	session, err := c.Client.NewSession()
	if err != nil {
		return err
	}
	c.Session = session

	stdin, err := session.StdinPipe()
	if err != nil {
		return err
	}
	c.Stdin = stdin

	stdout, err := session.StdoutPipe()
	if err != nil {
		return err
	}
	c.Stdout = stdout

	// Run agent. We assume it's in the home dir or path we deployed to.
	// We run it directly.
	if err := session.Start("./perssh-server"); err != nil {
		return err
	}

	return nil
}

// SendRequest sends a JSON request to the agent.
func (c *Client) SendRequest(req common.Request) error {
	b, err := json.Marshal(req)
	if err != nil {
		return err
	}
	// Append newline as delimiter
	b = append(b, '\n')
	_, err = c.Stdin.Write(b)
	return err
}

func (c *Client) GetStdout() io.Reader {
	return c.Stdout
}
