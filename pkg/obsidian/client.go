package obsidian

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os/exec"
	"sync"
)

type Config struct {
	Email    string
	Password string
	Vault    VaultConfig
}

func (c Config) Validate() error {
	if c.Vault.Path == "" || c.Vault.Name == "" {
		return fmt.Errorf("missing configuration for obsidian vault")
	}
	if c.Vault.Password == "" {
		return fmt.Errorf("missing vault password")
	}
	return nil
}

type Client struct {
	c Config

	// syncMu serializes every operation that touches the vault or its sync
	// registration. Overlapping `ob sync` processes against one vault path are
	// not safe, and unlinking or deleting underneath a running sync is worse.
	syncMu sync.Mutex
}

func NewClient(ctx context.Context, config Config) (*Client, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}

	c := &Client{
		c: config,
	}

	if err := c.login(ctx); err != nil {
		return nil, err
	}

	if err := c.syncSetup(ctx); err != nil {
		return nil, err
	}

	if err := c.sync(ctx); err != nil {
		return nil, err
	}

	return c, nil
}

var ErrFileNotFound = errors.New("file not found in obisidan vault")

func (c *Client) UploadFiles(ctx context.Context, files ...MarkdownFile) error {
	c.syncMu.Lock()
	defer c.syncMu.Unlock()

	for _, f := range files {
		if err := c.writeMarkdownFile(f); err != nil {
			return err
		}
	}

	return c.syncWithNoLock(ctx)
}

func (c *Client) ReadFile(ctx context.Context, fp FilePath) (MarkdownFile, error) {
	c.syncMu.Lock()
	defer c.syncMu.Unlock()

	return c.readMarkdownFile(fp)
}

func (c *Client) Close(ctx context.Context) error {
	return errors.Join(c.syncUnlink(ctx), c.logout(ctx), c.deleteVault())
}

func runOB(ctx context.Context, args ...string) error {
	cmd := exec.CommandContext(ctx, "ob", args...)

	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("running %v: %w: %v", args[0], err, string(out))
	} else {
		log.Println("ob out:", string(out))
	}

	return nil
}
