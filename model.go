package main

import (
	"gorm.io/gorm"
)

type Items struct {
	gorm.Model
	ItemID   int `gorm:"primarykey"`
	Title    string
	Album    string
	Artist   string
	Url      string
	AcrID    string
	Metadata ItemMetadata `gorm:"embedded"`
}

type ItemMetadata struct {
	DeezerURL     string
	DeezerID      string
	SoundcloudURL string
	SoundcloudID  string
	SpotifyURL    string
	SpotifyID     string
	YoutubeURL    string
	YoutubeID     string
	TidalURL      string
	TidalID       string
	ApplemusicURL string
	ApplemusicID  string
}
