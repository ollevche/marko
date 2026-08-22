package obsidian

func (c *Client) login() error {
	return runOB("login", "--email", c.c.Email, "--password", c.c.Password)
}

func (c *Client) logout() error {
	if err := runOB("logout"); err != nil {
		return err
	}
	return nil
}
