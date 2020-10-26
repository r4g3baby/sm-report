package main

import (
	"github.com/Acidic9/go-steam/steamapi"
	"github.com/Acidic9/go-steam/steamid"
	"github.com/patrickmn/go-cache"
	"strconv"
	"time"
)

type SteamUser struct {
	SteamID    steamid.ID64
	Name       string
	AvatarURL  string
	ProfileURL string
}

var userCache = cache.New(12*time.Hour, 1*time.Hour)

func GetSteamUser(steamID uint64) (*SteamUser, error) {
	strID := strconv.FormatUint(steamID, 10)
	if value, ok := userCache.Get(strID); ok {
		return value.(*SteamUser), nil
	}

	profile, err := steamapi.NewKey(config.SteamKey).GetSinglePlayerSummaries(steamID)
	if err != nil {
		return nil, err
	}

	steamUser := &SteamUser{
		SteamID:    steamid.NewID64(steamID),
		Name:       profile.PersonaName,
		AvatarURL:  profile.AvatarFull,
		ProfileURL: profile.ProfileURL,
	}

	userCache.Set(strID, steamUser, cache.DefaultExpiration)

	return steamUser, nil
}
