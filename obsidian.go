package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
)

type obsidianConfig struct {
	Email                 string
	Password              string
	VaultPath             string
	TranscriptsPathPrefix string
}

type obsidian struct {
	c obsidianConfig
}

func newObsidian(c obsidianConfig) (*obsidian, error) {
	ob := &obsidian{
		c: c,
	}

	if err := ob.login(); err != nil {
		return nil, err
	}

	if err := ob.sync(); err != nil {
		return nil, err
	}

	es, err := os.ReadDir(c.VaultPath)
	log.Println(err)
	for _, es := range es {
		log.Println(es.Name())
	}

	return ob, nil
}

func (ob *obsidian) close() error {
	return ob.logout()
}

func (ob *obsidian) syncTranscript(t transcript) error {
	if err := ob.writeTranscriptFile(t); err != nil {
		return err
	}

	if err := ob.sync(); err != nil {
		return err
	}

	return nil
}

func (ob *obsidian) getTranscriptFilepath(t transcript) string {
	y, m, d := t.MeetingTime.Date()

	filepath := fmt.Sprintf("%v/%v/%v/%v - %v/%v - %v/",
		ob.c.VaultPath, ob.c.TranscriptsPathPrefix,
		y, m, m.String(), d, t.MeetingTime.Weekday())

	return filepath
}

func (ob *obsidian) login() error {
	return runOB("login", "--email", ob.c.Email, "--password", ob.c.Password)
}

func (ob *obsidian) sync() error {
	return runOB("sync", "--path", ob.c.VaultPath)
}

func (ob *obsidian) logout() error {
	if err := runOB("logout"); err != nil {
		return err
	}

	err := exec.Command("rm", "-r", ob.c.VaultPath).Run()
	if err != nil {
		return fmt.Errorf("cleaning up vault after logout: %v", err)
	}

	return nil
}

func (ob *obsidian) writeTranscriptFile(t transcript) error {
	transcriptFilepath := ob.getTranscriptFilepath(t)

	err := os.MkdirAll(transcriptFilepath, 0o755)
	if err != nil {
		return fmt.Errorf("running mkdirall: %w", err)
	}

	f, err := os.Create(transcriptFilepath + t.filename())
	if err != nil {
		return fmt.Errorf("creating transcript file: %w", err)
	}
	defer f.Close()

	_, err = f.WriteString(t.body())
	if err != nil {
		return fmt.Errorf("writing transcript file: %w", err)
	}

	return nil
}

func runOB(args ...string) error {
	args = append(args, "--disable-gpu", "--disable-software-rasterizer")

	cmd := exec.Command("ob", args...)

	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("running %v: %w: %v", args[0], err, string(out))
	} else {
		log.Println("ob out:", string(out))
	}

	return nil
}
