package obsidian

import "context"

func (c *Client) login(ctx context.Context) error {
	return runOB(ctx, "login", "--email", c.c.Email, "--password", c.c.Password)
}

func (c *Client) logout(ctx context.Context) error {
	if err := runOB(ctx, "logout"); err != nil {
		return err
	}
	return nil
}
