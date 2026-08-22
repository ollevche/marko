package obsidian

import (
	"context"
	"fmt"
	"log"
	"os/exec"
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
}

func NewClient(config Config) (*Client, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}

	c := &Client{
		c: config,
	}

	if err := c.login(); err != nil {
		return nil, err
	}

	if err := c.syncSetup(); err != nil {
		return nil, err
	}

	if err := c.sync(); err != nil {
		return nil, err
	}

	return c, nil
}

func (c *Client) UploadFile(ctx context.Context, f MarkdownFile) error {
	if err := c.writeMarkdownFile(f); err != nil {
		return err
	}
	if err := c.sync(); err != nil {
		return err
	}
	return nil
}

func (c *Client) Close() error {
	if err := c.logout(); err != nil {
		return err
	}
	if err := c.deleteVaultFiles(); err != nil {
		return err
	}
	return nil
}

func runOB(args ...string) error {
	cmd := exec.Command("ob", args...)

	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("running %v: %w: %v", args[0], err, string(out))
	} else {
		log.Println("ob out:", string(out))
	}

	return nil
}
