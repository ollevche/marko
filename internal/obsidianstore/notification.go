package obsidianstore

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/ollevche/marko/internal/domain/notification"
	"github.com/ollevche/marko/pkg/obsidian"
)

func (s *Store) ReadNotificationState(ctx context.Context) (notification.State, error) {
	digest, err := s.readMarkdownFile(ctx, s.notificationDigestFilepath())
	if err != nil {
		return notification.State{}, err
	}

	digest.Content = strings.TrimSpace(digest.Content)
	if digest.Content == "" {
		return notification.State{}, nil
	}

	rawFetchedAt, err := s.readMarkdownFile(ctx, s.notificationFetchedAtFilepath())
	if err != nil {
		return notification.State{}, err
	}

	fetchedAt, _ := time.Parse(time.RFC3339, rawFetchedAt.Content)
	if fetchedAt.IsZero() {
		return notification.State{}, err
	}

	return notification.State{Digest: digest.Content, FetchedAt: fetchedAt}, nil
}

func (s *Store) UploadNotificationSummary(ctx context.Context, summary notification.Summary) error {
	files := []obsidian.MarkdownFile{
		{
			FilePath: s.notificationDigestFilepath(),
			Content:  summary.Digest,
		},
		{
			FilePath: s.notificationFetchedAtFilepath(),
			Content:  summary.FetchedAt.Format(time.RFC3339),
		},
		{
			FilePath: s.notificationSummaryFilepath(),
			Content:  newNotificationSummaryMarkdownContent(summary),
		},
	}

	if err := s.c.UploadFiles(ctx, files...); err != nil {
		return fmt.Errorf("uploading notification files: %w", err)
	}

	return nil
}

func newNotificationSummaryMarkdownContent(summary notification.Summary) string {
	var totalUnread int
	for _, n := range summary.Notifications {
		if n.Unread {
			totalUnread++
		}
	}

	var b strings.Builder

	fmt.Fprintf(&b, "Updated At: %s\n", summary.FetchedAt.Format(time.DateTime))
	fmt.Fprintf(&b, "Unread: %d\n", totalUnread)
	fmt.Fprintf(&b, "Total: %d\n\n", len(summary.Notifications))

	byRepo := make(map[string][]notification.Notification)
	for _, n := range summary.Notifications {
		byRepo[n.Repo.FullName] = append(byRepo[n.Repo.FullName], n)
	}

	repoFullNames := make([]string, 0, len(byRepo))
	for repoFullName := range byRepo {
		repoFullNames = append(repoFullNames, repoFullName)
	}

	repoFullNames = sort.StringSlice(repoFullNames)

	for i, repoFullName := range repoFullNames {
		fmt.Fprintf(&b, "[%s](%s):\n", repoFullName, byRepo[repoFullName][0].Repo.URL)
		for _, n := range byRepo[repoFullName] {
			fmt.Fprintf(&b, "* %s: %s", n.Type, n.Title)
			if n.Unread {
				fmt.Fprintf(&b, " (new)")
			}
			fmt.Fprintf(&b, "\n")
		}
		if i+1 == len(repoFullNames) {
			fmt.Fprintf(&b, "\n")
		}
	}

	return b.String()
}

func (s *Store) notificationDigestFilepath() obsidian.FilePath {
	return obsidian.FilePath{
		Folders:  s.notificationFolders,
		Filename: "digest.md",
	}
}

func (s *Store) notificationSummaryFilepath() obsidian.FilePath {
	return obsidian.FilePath{
		Folders:  s.notificationFolders,
		Filename: "summary.md",
	}
}

func (s *Store) notificationFetchedAtFilepath() obsidian.FilePath {
	return obsidian.FilePath{
		Folders:  s.notificationFolders,
		Filename: "fetched_at.md",
	}
}
