package storage

import (
	"database/sql"
	"log"
)

type Feed struct {
	Id          int64   `json:"id"`
	FolderId    *int64  `json:"folder_id"`
	Title       string  `json:"title"`
	Description string  `json:"description"`
	Link        string  `json:"link"`
	FeedLink    string  `json:"feed_link"`
	Icon        *[]byte `json:"icon,omitempty"`
	HasIcon     bool    `json:"has_icon"`
	CustomOrder string  `json:"custom_order"`
}

func (s *Storage) CreateFeed(title, description, link, feedLink, customOrder string, folderId *int64) *Feed {
	if title == "" {
		title = feedLink
	}
	if customOrder == "" {
		// it's a text column (not int), to provide a bit more flexibility

		// it's a not null column that defaults to an empty string (both in go
		// and sql), to make the order-by result not having to mix null and not
		// null

		// OPML imports should include customOrder="a" or customOrder="1" or
		// such, but when adding feeds via other means, we should just default
		// to something. if we default to "", we couldn't sort via ASC

		// defaulting to a "low" sort here seems more important, so that feeds
		// imported with any order through the OPML, is more likely to be
		// respected. "xxxxxxxxx" should be "low enough" to not conflict with
		// feeds without a custom order

		customOrder = "xxxxxxxxx"
	}
	row := s.db.QueryRow(`
		insert into feeds (title, description, link, feed_link, folder_id, custom_order)
		values (?, ?, ?, ?, ?, ?)
		on conflict (feed_link) do update set folder_id = ?
        returning id`,
		title, description, link, feedLink, folderId, customOrder,
		folderId,
	)

	var id int64
	err := row.Scan(&id)
	if err != nil {
		log.Print(err)
		return nil
	}
	return &Feed{
		Id:          id,
		Title:       title,
		Description: description,
		Link:        link,
		FeedLink:    feedLink,
		FolderId:    folderId,
		CustomOrder: customOrder,
	}
}

func (s *Storage) DeleteFeed(feedId int64) bool {
	result, err := s.db.Exec(`delete from feeds where id = ?`, feedId)
	if err != nil {
		log.Print(err)
		return false
	}
	nrows, err := result.RowsAffected()
	if err != nil {
		if err != sql.ErrNoRows {
			log.Print(err)
		}
		return false
	}
	return nrows == 1
}

func (s *Storage) RenameFeed(feedId int64, newTitle string) bool {
	_, err := s.db.Exec(`update feeds set title = ? where id = ?`, newTitle, feedId)
	return err == nil
}

func (s *Storage) UpdateFeedFolder(feedId int64, newFolderId *int64) bool {
	_, err := s.db.Exec(`update feeds set folder_id = ? where id = ?`, newFolderId, feedId)
	return err == nil
}

func (s *Storage) UpdateFeedLink(feedId int64, newLink string) bool {
	_, err := s.db.Exec(`update feeds set feed_link = ? where id = ?`, newLink, feedId)
	return err == nil
}

func (s *Storage) UpdateFeedIcon(feedId int64, icon *[]byte) bool {
	_, err := s.db.Exec(`update feeds set icon = ? where id = ?`, icon, feedId)
	return err == nil
}

func (s *Storage) ListFeeds() []Feed {
	result := make([]Feed, 0)
	rows, err := s.db.Query(`
		select id, folder_id, title, description, link, feed_link,
		       ifnull(length(icon), 0) > 0 as has_icon
		from feeds
		order by title collate nocase
	`)
	if err != nil {
		log.Print(err)
		return result
	}
	for rows.Next() {
		var f Feed
		err = rows.Scan(
			&f.Id,
			&f.FolderId,
			&f.Title,
			&f.Description,
			&f.Link,
			&f.FeedLink,
			&f.HasIcon,
		)
		if err != nil {
			log.Print(err)
			return result
		}
		result = append(result, f)
	}
	return result
}

func (s *Storage) ListFeedsMissingIcons() []Feed {
	result := make([]Feed, 0)
	rows, err := s.db.Query(`
		select id, folder_id, title, description, link, feed_link
		from feeds
		where icon is null
	`)
	if err != nil {
		log.Print(err)
		return result
	}
	for rows.Next() {
		var f Feed
		err = rows.Scan(
			&f.Id,
			&f.FolderId,
			&f.Title,
			&f.Description,
			&f.Link,
			&f.FeedLink,
		)
		if err != nil {
			log.Print(err)
			return result
		}
		result = append(result, f)
	}
	return result
}

func (s *Storage) GetFeed(id int64) *Feed {
	var f Feed
	err := s.db.QueryRow(`
		select
			id, folder_id, title, link, feed_link,
			icon, ifnull(icon, '') != '' as has_icon
		from feeds where id = ?
	`, id).Scan(
		&f.Id, &f.FolderId, &f.Title, &f.Link, &f.FeedLink,
		&f.Icon, &f.HasIcon,
	)
	if err != nil {
		if err != sql.ErrNoRows {
			log.Print(err)
		}
		return nil
	}
	return &f
}

func (s *Storage) ResetFeedErrors() {
	if _, err := s.db.Exec(`delete from feed_errors`); err != nil {
		log.Print(err)
	}
}

func (s *Storage) SetFeedError(feedID int64, lastError error) {
	_, err := s.db.Exec(`
		insert into feed_errors (feed_id, error)
		values (?, ?)
		on conflict (feed_id) do update set error = excluded.error`,
		feedID, lastError.Error(),
	)
	if err != nil {
		log.Print(err)
	}
}

func (s *Storage) GetFeedErrors() map[int64]string {
	errors := make(map[int64]string)

	rows, err := s.db.Query(`select feed_id, error from feed_errors`)
	if err != nil {
		log.Print(err)
		return errors
	}

	for rows.Next() {
		var id int64
		var error string
		if err = rows.Scan(&id, &error); err != nil {
			log.Print(err)
		}
		errors[id] = error
	}
	return errors
}

func (s *Storage) SetFeedSize(feedId int64, size int) {
	_, err := s.db.Exec(`
		insert into feed_sizes (feed_id, size)
		values (?, ?)
		on conflict (feed_id) do update set size = excluded.size`,
		feedId, size,
	)
	if err != nil {
		log.Print(err)
	}
}
