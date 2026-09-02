package githubsource

import (
	"context"
	"fmt"
	"time"

	"github.com/google/go-github/v90/github"
	"github.com/ollevche/marko/internal/domain/notification"
)

type Config struct {
	Token string
}

type Source struct {
	c *github.Client
}

func New(c Config) (*Source, error) {
	client, err := github.NewClient(github.WithAuthToken(c.Token),
		github.WithTimeout(30*time.Second))
	if err != nil {
		return nil, fmt.Errorf("creating github client: %w", err)
	}

	s := &Source{c: client}
	return s, nil
}

func (s *Source) ListNotifications(ctx context.Context) ([]notification.Notification, error) {
	var respNotifications []*github.Notification

	const perPage = 50

	opts := &github.NotificationListOptions{
		All: true,
		ListOptions: github.ListOptions{
			PerPage: perPage,
		},
	}

	for {
		paginated, resp, err := s.c.Activity.ListNotifications(ctx, opts)
		if err != nil {
			return nil, fmt.Errorf("requesting notifications: %w", err)
		}

		respNotifications = append(respNotifications, paginated...)

		if resp.NextPage == 0 {
			break
		}

		opts.Page = resp.NextPage
	}

	notifications := make([]notification.Notification, len(respNotifications))
	for i, rn := range respNotifications {
		var n notification.Notification

		if rn.ID != nil {
			n.ID = *rn.ID
		}
		if rn.Repository != nil {
			if rn.Repository.FullName != nil {
				n.Repo.FullName = *rn.Repository.FullName
			}
			if rn.Repository.HTMLURL != nil {
				n.Repo.URL = *rn.Repository.HTMLURL
			}
		}
		if rn.Unread != nil {
			n.Unread = *rn.Unread
		}
		if rn.Subject != nil {
			if rn.Subject.Title != nil {
				n.Title = *rn.Subject.Title
			}
			if rn.Subject.Type != nil {
				n.Type = *rn.Subject.Type
			}
		}

		notifications[i] = n
	}

	return notifications, nil
}
