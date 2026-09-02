package obsidianstore

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/ollevche/marko/internal/domain/transcript"
	"github.com/ollevche/marko/pkg/obsidian"
)

func (s *Store) UploadTranscript(ctx context.Context, t transcript.Transcript) error {
	err := s.c.UploadFiles(ctx, newMarkdownFileFromTranscript(s.transcriptFolders, t))
	if err != nil {
		return fmt.Errorf("uploading file: %w", err)
	}

	return nil
}

func newMarkdownFileFromTranscript(transcriptFolders []string, t transcript.Transcript) obsidian.MarkdownFile {
	mt := t.MeetingTime
	y, m, d := t.MeetingTime.Date()

	folders := append(transcriptFolders, []string{
		fmt.Sprintf("%v", y),
		fmt.Sprintf("%02d - %v", m, m.String()),
		fmt.Sprintf("%02d - %v", d, t.MeetingTime.Weekday()),
	}...)

	filename := fmt.Sprintf("%02d-%02d %s.md",
		t.MeetingTime.Hour(), t.MeetingTime.Minute(), t.Title)

	var content strings.Builder

	content.WriteString("Date: ")
	content.WriteString(mt.Format(time.DateTime))
	content.WriteByte('\n')

	content.WriteString("Duration: ")
	content.WriteString(t.Duration.String())
	content.WriteByte('\n')

	content.WriteString("Attendees: ")
	att := "n/a"
	if len(t.Attendees) != 0 {
		att = strings.Join(t.Attendees, ", ")
	}
	content.WriteString(att)
	content.WriteString("\n\n")

	for i, l := range t.Lines {
		content.WriteString(l.Speaker)
		content.WriteByte('\n')

		content.WriteString(l.Text)
		if len(t.Lines) != i+1 {
			content.WriteString("\n\n")
		}
	}

	return obsidian.MarkdownFile{
		FilePath: obsidian.FilePath{
			Folders:  folders,
			Filename: filename,
		},
		Content: content.String(),
	}
}
