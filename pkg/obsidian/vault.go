package obsidian

import (
	"context"
	"fmt"
	"os"
	"path"
	"strings"
)

type VaultConfig struct {
	Path     string
	Name     string
	Password string
}

func (c *Client) syncSetup(ctx context.Context) error {
	return runOB(ctx, "sync-setup", "--vault", c.c.Vault.Name, "--password", c.c.Vault.Password,
		"--path", c.c.Vault.Path, "--device-name", "marko")
}

func (c *Client) sync(ctx context.Context) error {
	c.syncMu.Lock()
	defer c.syncMu.Unlock()

	return runOB(ctx, "sync", "--path", c.c.Vault.Path)
}

func (c *Client) syncUnlink(ctx context.Context) error {
	c.syncMu.Lock()
	defer c.syncMu.Unlock()

	return runOB(ctx, "sync-unlink", "--path", c.c.Vault.Path)
}

func (c *Client) deleteVault() error {
	c.syncMu.Lock()
	defer c.syncMu.Unlock()

	if err := os.RemoveAll(c.c.Vault.Path); err != nil {
		return fmt.Errorf("cleaning up vault after logout: %w", err)
	}
	return nil
}

type MarkdownFile struct {
	Folders  []string
	Filename string
	Content  string
}

func (c *Client) folderPath(f MarkdownFile) string {
	sanitized := make([]string, len(f.Folders)+1)
	sanitized[0] = c.c.Vault.Path
	for i, f := range f.Folders {
		sanitized[i+1] = sanitize(f)
	}
	return path.Join(sanitized...)
}

func (c *Client) filepath(f MarkdownFile) string {
	return path.Join(c.folderPath(f), sanitize(f.Filename))
}

func (c *Client) writeMarkdownFile(mf MarkdownFile) error {
	err := os.MkdirAll(c.folderPath(mf), 0o755)
	if err != nil {
		return fmt.Errorf("running mkdirall: %w", err)
	}

	f, err := os.Create(c.filepath(mf))
	if err != nil {
		return fmt.Errorf("creating transcript file: %w", err)
	}
	defer f.Close()

	_, err = f.WriteString(mf.Content)
	if err != nil {
		return fmt.Errorf("writing transcript file: %w", err)
	}

	return nil
}

func sanitize(name string) string {
	cleaned := strings.Map(func(r rune) rune {
		switch {
		case r < 0x20 || r == 0x7f: // control characters
			return ' '
		case strings.ContainsRune(`\/:*?"<>|#^[]`, r):
			return ' '
		default:
			return r
		}
	}, name)

	cleaned = strings.Join(strings.Fields(cleaned), " ")
	// Leading dots hide the file, trailing dots are dropped by some filesystems.
	cleaned = strings.Trim(cleaned, ".")

	if cleaned == "" {
		return "untitled"
	}

	return cleaned
}
