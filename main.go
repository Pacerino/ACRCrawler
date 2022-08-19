package main

import (
	"fmt"
	"net/url"
	"os"
	"path"
	"regexp"
	"strings"

	"github.com/go-rod/rod"
	"github.com/joho/godotenv"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

type CrawlSession struct {
	db       *gorm.DB
	itemChan chan Items
}

type LinksIds struct {
	// links
	SpotifyLink string
	DeezerLink  string
	YouTubeLink string
	// ids
	SpotifyID string
	DeezerID  string
	YoutubeID string
}

func init() {
	if err := godotenv.Load(); err != nil && !os.IsNotExist(err) {
		panic(err)
	}

	if len(os.Getenv("LOG_LEVEL")) > 0 {
		switch strings.ToLower(os.Getenv("LOG_LEVEL")) {
		case "error":
			logrus.SetLevel(logrus.ErrorLevel)
		case "fatal":
			logrus.SetLevel(logrus.FatalLevel)
		case "info":
			logrus.SetLevel(logrus.InfoLevel)
		case "debug":
			logrus.SetLevel(logrus.DebugLevel)
		default:
			logrus.SetLevel(logrus.InfoLevel)
		}
	}

	for _, env := range []string{"DB_HOST", "DB_USER", "DB_PASS", "DB_DATABASE", "DB_PORT", "DB_SSL"} {
		if len(os.Getenv(env)) == 0 {
			logrus.Fatal(fmt.Sprintf("Missing %s from environment", env))
		}
	}
}

func main() {
	db, err := connectDB(fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=%s",
		os.Getenv("DB_HOST"),
		os.Getenv("DB_USER"),
		os.Getenv("DB_PASS"),
		os.Getenv("DB_DATABASE"),
		os.Getenv("DB_PORT"),
		os.Getenv("DB_SSL"),
	))
	if err != nil {
		logrus.WithError(err).Error("Error while connecting to a database!")
	}

	ss := CrawlSession{
		db:       db,
		itemChan: make(chan Items),
	}
	ss.itemWorker()

}

func (s *CrawlSession) itemWorker() {
	var items []Items
	itemsResp := s.db.Where("length(acr_id) > 0 and (length(spotify_url) > 0 or length(deezer_url) > 0 or length(youtube_url) > 0) is not true").Find(&items)
	logrus.Debug("get all items")
	if itemsResp.Error != nil {
		logrus.Error("could not retrieve Items from DB")
	}
	for _, item := range items {
		s.handleItem(item)
	}
}

func (s *CrawlSession) handleItem(item Items) {
	var foundLinks LinksIds
	if len(item.AcrID) > 0 {
		reqLink := fmt.Sprintf(`https://aha-music.com/%s`, item.AcrID)
		page := rod.New().ControlURL("ws://127.0.0.1:3000").MustConnect().MustPage(reqLink)
		section := page.MustWaitLoad().MustElements("a.block")
		spotifyRegex, err := regexp.Compile(`(?mi)^(https:\/\/open.spotify.com\/track\/)(.*)$`)
		if err != nil {
			logrus.WithError(err).Fatal("Could not compile Regex for Spotify!")
		}

		deezerRegex, err := regexp.Compile(`(?mi)^https?:\/\/(?:www\.)?deezer\.com\/(track|album|playlist)\/(\d+)$`)
		if err != nil {
			logrus.WithError(err).Fatal("Could not compile Regex for Deezer!")
		}

		youtubeRegex, err := regexp.Compile(`^((?:https?:)?\/\/)?((?:www|m|music)\.)?((?:youtube\.com|youtu.be))(\/(?:[\w\-]+\?v=|embed\/|v\/)?)([\w\-]+)(\S+)?$`)
		if err != nil {
			logrus.WithError(err).Fatal("Could not compile Regex for YouTube!")
		}

		for _, s := range section {
			link := s.MustProperty("href").String()
			if spotifyLink := spotifyRegex.FindString(link); len(spotifyLink) > 0 {
				foundLinks.SpotifyLink = spotifyLink
				foundLinks.SpotifyID = path.Base(spotifyLink)
			}

			if deezerLink := deezerRegex.FindString(link); len(deezerLink) > 0 {
				foundLinks.DeezerLink = deezerLink
				foundLinks.DeezerID = path.Base(deezerLink)
			}

			if youtubeLink := youtubeRegex.FindString(link); len(youtubeLink) > 0 {
				foundLinks.YouTubeLink = youtubeLink
				u, err := url.Parse(youtubeLink)
				if err != nil {
					logrus.WithError(err).Errorln("could not get url queries from youtube url")
				}
				queryParams := u.Query()
				foundLinks.YoutubeID = queryParams.Get("v")
			}
		}
		logrus.Info(fmt.Sprintf("Found Links for %s", item.AcrID))
		s.updateItem(item, foundLinks)

	} else {
		logrus.Info("Skip empty acrID!")
	}
}

func (s *CrawlSession) updateItem(item Items, data LinksIds) {
	upItem := Items{
		Metadata: ItemMetadata{
			SpotifyURL: data.SpotifyLink,
			SpotifyID:  data.SpotifyID,
			DeezerURL:  data.DeezerLink,
			DeezerID:   data.DeezerID,
			YoutubeURL: data.YouTubeLink,
			YoutubeID:  data.YoutubeID,
		},
	}
	result := s.db.Model(&item).Updates(upItem)
	if result.Error != nil {
		logrus.WithError(result.Error).Fatalln("could not update item!")
		os.Exit(0)
	}
	logrus.Info(fmt.Sprintf("Updated Item with Item ID %d, rows affected %d", item.ItemID, result.RowsAffected))
}
