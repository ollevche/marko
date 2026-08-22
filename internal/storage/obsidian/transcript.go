package obsidian

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/ollevche/marko/internal/domain/transcript"
	"github.com/ollevche/marko/pkg/obsidian"
)

func (s *Storage) UploadTranscript(ctx context.Context, t transcript.Transcript) error {
	mt := t.MeetingTime
	y, m, d := t.MeetingTime.Date()

	folders := append(s.transcriptFolders, []string{
		fmt.Sprintf("%v", y),
		fmt.Sprintf("%d - %v", m, m.String()),
		fmt.Sprintf("%v - %v", d, t.MeetingTime.Weekday()),
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

	err := s.c.UploadFile(ctx, obsidian.MarkdownFile{
		Folders:  folders,
		Filename: filename,
		Content:  content.String(),
	})
	if err != nil {
		return fmt.Errorf("uploading file: %w", err)
	}

	return nil
}
