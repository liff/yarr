package worker

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/nkanaev/yarr/src/storage"
)

const NUM_WORKERS = 4

type Worker struct {
	db      *storage.Storage
	pending *int32
	refresh *time.Ticker
	reflock sync.Mutex
	stopper chan bool
}

func NewWorker(db *storage.Storage) *Worker {
	pending := int32(0)
	return &Worker{db: db, pending: &pending}
}

func (w *Worker) FeedsPending() int32 {
	return *w.pending
}

func (w *Worker) StartFeedCleaner() {
	go w.db.DeleteOldItems()
	ticker := time.NewTicker(time.Hour * 24)
	go func() {
		for {
			<-ticker.C
			w.db.DeleteOldItems()
		}
	}()
}

func (w *Worker) FindFavicons() {
	go func() {
		for _, feed := range w.db.ListFeedsMissingIcons() {
			w.FindFeedFavicon(feed)
		}
	}()
}

func (w *Worker) FindFeedFavicon(feed storage.Feed) {
	icon, err := findFavicon(feed.Link, feed.FeedLink)
	if err != nil {
		log.Error().Err(err).Msgf("Failed to find favicon for %s (%s)", feed.FeedLink, feed.Link)
	}
	if icon != nil {
		w.db.UpdateFeedIcon(feed.Id, icon)
	}
}

func (w *Worker) SetRefreshRate(minute int64) {
	if w.stopper != nil {
		w.refresh.Stop()
		w.refresh = nil
		w.stopper <- true
		w.stopper = nil
	}

	if minute == 0 {
		return
	}

	w.stopper = make(chan bool)
	w.refresh = time.NewTicker(time.Minute * time.Duration(minute))

	go func(fire <-chan time.Time, stop <-chan bool, m int64) {
		log.Info().Msgf("auto-refresh %dm: starting", m)
		for {
			select {
			case <-fire:
				log.Info().Msgf("auto-refresh %dm: firing", m)
				w.RefreshFeeds()
			case <-stop:
				log.Info().Msgf("auto-refresh %dm: stopping", m)
				return
			}
		}
	}(w.refresh.C, w.stopper, minute)
}

func (w *Worker) RefreshFeeds() {
	w.reflock.Lock()
	defer w.reflock.Unlock()

	if *w.pending > 0 {
		log.Info().Msg("Refreshing already in progress")
		return
	}

	feeds := w.db.ListFeeds()
	if len(feeds) == 0 {
		log.Info().Msg("Nothing to refresh")
		return
	}

	log.Info().Msg("Refreshing feeds")
	atomic.StoreInt32(w.pending, int32(len(feeds)))
	go w.refresher(feeds)
}

func (w *Worker) refresher(feeds []storage.Feed) {
	w.db.ResetFeedErrors()

	srcqueue := make(chan storage.Feed, len(feeds))
	dstqueue := make(chan []storage.Item)

	for i := 0; i < NUM_WORKERS; i++ {
		go w.worker(srcqueue, dstqueue)
	}

	for _, feed := range feeds {
		srcqueue <- feed
	}
	for i := 0; i < len(feeds); i++ {
		items := <-dstqueue
		if len(items) > 0 {
			w.db.CreateItems(items)
			w.db.SetFeedSize(items[0].FeedId, len(items))
		}
		atomic.AddInt32(w.pending, -1)
		w.db.SyncSearch()
	}
	close(srcqueue)
	close(dstqueue)

	log.Info().Msgf("Finished refreshing %d feeds", len(feeds))
}

func (w *Worker) worker(srcqueue <-chan storage.Feed, dstqueue chan<- []storage.Item) {
	for feed := range srcqueue {
		items, err := listItems(feed, w.db)
		if err != nil {
			w.db.SetFeedError(feed.Id, err)
		}
		dstqueue <- items
	}
}
