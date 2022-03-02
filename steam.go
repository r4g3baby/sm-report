package sm_report

import (
	"fmt"
	"github.com/Acidic9/go-steam/steamapi"
	"github.com/Acidic9/go-steam/steamid"
	"github.com/fanjindong/go-cache"
	"github.com/r4g3baby/sm-report/config"
	"strconv"
	"time"
)

type steamUser struct {
	SteamID    steamid.ID64
	Name       string
	AvatarURL  string
	ProfileURL string
}

var userCache = cache.NewMemCache(cache.WithClearInterval(1 * time.Hour))

func getSteamUser(steamID uint64) (*steamUser, error) {
	strID := strconv.FormatUint(steamID, 10)
	if value, ok := userCache.Get(strID); ok {
		return value.(*steamUser), nil
	}

	profile, err := steamapi.NewKey(config.Config.SteamKey).GetSinglePlayerSummaries(steamID)
	if err != nil {
		return nil, err
	}

	steamUser := &steamUser{
		SteamID:    steamid.NewID64(steamID),
		Name:       profile.PersonaName,
		AvatarURL:  profile.AvatarFull,
		ProfileURL: fmt.Sprintf("https://steamcommunity.com/profiles/%s", strID),
	}

	userCache.Set(strID, steamUser, cache.WithEx(12*time.Hour))

	return steamUser, nil
}
