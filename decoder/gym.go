package decoder

import (
	"database/sql"
	"fmt"
	"github.com/google/go-cmp/cmp"
	"github.com/jellydator/ttlcache/v3"
	"github.com/jmoiron/sqlx"
	log "github.com/sirupsen/logrus"
	"golbat/pogo"
	"golbat/util"
	"golbat/webhooks"
	"gopkg.in/guregu/null.v4"
	"time"
)

type Gym struct {
	Id                    string      `db:"id"`
	Lat                   float64     `db:"lat"`
	Lon                   float64     `db:"lon"`
	Name                  null.String `db:"name"`
	Url                   null.String `db:"url"`
	LastModifiedTimestamp null.Int    `db:"last_modified_timestamp"`
	RaidEndTimestamp      null.Int    `db:"raid_end_timestamp"`
	RaidSpawnTimestamp    null.Int    `db:"raid_spawn_timestamp"`
	RaidBattleTimestamp   null.Int    `db:"raid_battle_timestamp"`
	Updated               int64       `db:"updated"`
	RaidPokemonId         null.Int    `db:"raid_pokemon_id"`
	GuardingPokemonId     null.Int    `db:"guarding_pokemon_id"`
	AvailableSlots        null.Int    `db:"available_slots"`
	TeamId                null.Int    `db:"team_id"`
	RaidLevel             null.Int    `db:"raid_level"`
	Enabled               null.Int    `db:"enabled"`
	ExRaidEligible        null.Int    `db:"ex_raid_eligible"`
	InBattle              null.Int    `db:"in_battle"`
	RaidPokemonMove1      null.Int    `db:"raid_pokemon_move_1"`
	RaidPokemonMove2      null.Int    `db:"raid_pokemon_move_2"`
	RaidPokemonForm       null.Int    `db:"raid_pokemon_form"`
	RaidPokemonCp         null.Int    `db:"raid_pokemon_cp"`
	RaidIsExclusive       null.Int    `db:"raid_is_exclusive"`
	CellId                null.Int    `db:"cell_id"`
	Deleted               int8        `db:"deleted"`
	TotalCp               null.Int    `db:"total_cp"`
	FirstSeenTimestamp    int64       `db:"first_seen_timestamp"`
	RaidPokemonGender     null.Int    `db:"raid_pokemon_gender"`
	SponsorId             null.Int    `db:"sponsor_id"`
	PartnerId             null.String `db:"partner_id"`
	RaidPokemonCostume    null.Int    `db:"raid_pokemon_costume"`
	RaidPokemonEvolution  null.Int    `db:"raid_pokemon_evolution"`
	ArScanEligible        null.Int    `db:"ar_scan_eligible"`
	PowerUpLevel          null.Int    `db:"power_up_level"`
	PowerUpPoints         null.Int    `db:"power_up_points"`
	PowerUpEndTimestamp   null.Int    `db:"power_up_end_timestamp"`

	//`id` varchar(35) NOT NULL,
	//`lat` double(18,14) NOT NULL,
	//`lon` double(18,14) NOT NULL,
	//`name` varchar(128) DEFAULT NULL,
	//`url` varchar(200) DEFAULT NULL,
	//`last_modified_timestamp` int unsigned DEFAULT NULL,
	//`raid_end_timestamp` int unsigned DEFAULT NULL,
	//`raid_spawn_timestamp` int unsigned DEFAULT NULL,
	//`raid_battle_timestamp` int unsigned DEFAULT NULL,
	//`updated` int unsigned NOT NULL,
	//`raid_pokemon_id` smallint unsigned DEFAULT NULL,
	//`guarding_pokemon_id` smallint unsigned DEFAULT NULL,
	//`available_slots` smallint unsigned DEFAULT NULL,
	//`availble_slots` smallint unsigned GENERATED ALWAYS AS (`available_slots`) VIRTUAL,
	//`team_id` tinyint unsigned DEFAULT NULL,
	//`raid_level` tinyint unsigned DEFAULT NULL,
	//`enabled` tinyint unsigned DEFAULT NULL,
	//`ex_raid_eligible` tinyint unsigned DEFAULT NULL,
	//`in_battle` tinyint unsigned DEFAULT NULL,
	//`raid_pokemon_move_1` smallint unsigned DEFAULT NULL,
	//`raid_pokemon_move_2` smallint unsigned DEFAULT NULL,
	//`raid_pokemon_form` smallint unsigned DEFAULT NULL,
	//`raid_pokemon_cp` int unsigned DEFAULT NULL,
	//`raid_is_exclusive` tinyint unsigned DEFAULT NULL,
	//`cell_id` bigint unsigned DEFAULT NULL,
	//`deleted` tinyint unsigned NOT NULL DEFAULT '0',
	//`total_cp` int unsigned DEFAULT NULL,
	//`first_seen_timestamp` int unsigned NOT NULL,
	//`raid_pokemon_gender` tinyint unsigned DEFAULT NULL,
	//`sponsor_id` smallint unsigned DEFAULT NULL,
	//`partner_id` varchar(35) DEFAULT NULL,
	//`raid_pokemon_costume` smallint unsigned DEFAULT NULL,
	//`raid_pokemon_evolution` tinyint unsigned DEFAULT NULL,
	//`ar_scan_eligible` tinyint unsigned DEFAULT NULL,
	//`power_up_level` smallint unsigned DEFAULT NULL,
	//`power_up_points` int unsigned DEFAULT NULL,
	//`power_up_end_timestamp` int unsigned DEFAULT NULL,
}

//
//SELECT CONCAT("'", GROUP_CONCAT(column_name ORDER BY ordinal_position SEPARATOR "', '"), "'") AS columns
//FROM information_schema.columns
//WHERE table_schema = 'db_name' AND table_name = 'tbl_name'
//
//SELECT CONCAT("'", GROUP_CONCAT(column_name ORDER BY ordinal_position SEPARATOR "', '"), " = ", "'") AS columns
//FROM information_schema.columns
//WHERE table_schema = 'db_name' AND table_name = 'tbl_name'

func getGymRecord(db *sqlx.DB, fortId string) (*Gym, error) {
	inMemoryGym := gymCache.Get(fortId)
	if inMemoryGym != nil {
		gym := inMemoryGym.Value()
		return &gym, nil
	}
	gym := Gym{}

	err := db.Get(&gym, "SELECT id, lat, lon, name, url,last_modified_timestamp, raid_end_timestamp, raid_spawn_timestamp, raid_battle_timestamp, updated, raid_pokemon_id, guarding_pokemon_id, available_slots, team_id, raid_level, enabled, ex_raid_eligible, in_battle, raid_pokemon_move_1, raid_pokemon_move_2, raid_pokemon_form, raid_pokemon_cp, raid_is_exclusive, cell_id, deleted, total_cp, first_seen_timestamp, raid_pokemon_gender, sponsor_id, partner_id, raid_pokemon_costume, raid_pokemon_evolution, ar_scan_eligible, power_up_level, power_up_points, power_up_end_timestamp FROM gym WHERE id = ?", fortId)

	if err == sql.ErrNoRows {
		return nil, nil
	}

	if err != nil {
		return nil, err
	}

	gymCache.Set(fortId, gym, ttlcache.DefaultTTL)
	return &gym, nil
}

func calculatePowerUpPoints(fortData *pogo.PokemonFortProto) (null.Int, null.Int) {
	now := time.Now().Unix()
	powerUpLevelExpirationMs := int64(fortData.PowerUpLevelExpirationMs) / 1000
	powerUpPoints := int64(fortData.PowerUpProgressPoints)
	powerUpLevel := null.IntFrom(0)
	powerUpEndTimestamp := null.NewInt(0, false)
	if powerUpPoints < 50 {
		powerUpLevel = null.IntFrom(0)
	} else if powerUpPoints < 100 && powerUpLevelExpirationMs > now {
		powerUpLevel = null.IntFrom(1)
		powerUpEndTimestamp = null.IntFrom(powerUpLevelExpirationMs)
	} else if powerUpPoints < 150 && powerUpLevelExpirationMs > now {
		powerUpLevel = null.IntFrom(2)
		powerUpEndTimestamp = null.IntFrom(powerUpLevelExpirationMs)
	} else if powerUpLevelExpirationMs > now {
		powerUpLevel = null.IntFrom(3)
		powerUpEndTimestamp = null.IntFrom(powerUpLevelExpirationMs)
	} else {
		powerUpLevel = null.IntFrom(0)
	}

	return powerUpLevel, powerUpEndTimestamp
}

func (gym *Gym) updateGymFromFort(fortData *pogo.PokemonFortProto, cellId uint64) *Gym {
	gym.Id = fortData.FortId
	gym.Lat = fortData.Latitude  //fmt.Sprintf("%f", fortData.Latitude)
	gym.Lon = fortData.Longitude //fmt.Sprintf("%f", fortData.Longitude)
	gym.Enabled = null.IntFrom(util.BoolToInt[int64](fortData.Enabled))
	gym.GuardingPokemonId = null.IntFrom(int64(fortData.GuardPokemonId))
	gym.TeamId = null.IntFrom(int64(fortData.Team))
	gym.AvailableSlots = null.IntFrom(int64(fortData.GymDisplay.SlotsAvailable))
	gym.LastModifiedTimestamp = null.IntFrom(fortData.LastModifiedMs / 1000)
	gym.ExRaidEligible = null.IntFrom(util.BoolToInt[int64](fortData.IsArScanEligible))

	if fortData.ImageUrl != "" {
		gym.Url = null.StringFrom(fortData.ImageUrl)
	}
	gym.InBattle = null.IntFrom(util.BoolToInt[int64](fortData.IsInBattle))
	gym.ArScanEligible = null.IntFrom(util.BoolToInt[int64](fortData.IsArScanEligible))
	gym.PowerUpPoints = null.IntFrom(int64(fortData.PowerUpProgressPoints))

	gym.PowerUpLevel, gym.PowerUpEndTimestamp = calculatePowerUpPoints(fortData)

	if fortData.PartnerId == "" {
		gym.PartnerId = null.NewString("", false)
	} else {
		gym.PartnerId = null.StringFrom(fortData.PartnerId)
	}

	if fortData.ImageUrl != "" {
		gym.Url = null.StringFrom(fortData.ImageUrl)

	}
	if fortData.Team == 0 { // check!!
		gym.TotalCp = null.IntFrom(0)
	} else {
		totalCp := int64(fortData.GymDisplay.TotalGymCp)
		if gym.TotalCp.Int64-totalCp > 100 || totalCp-gym.TotalCp.Int64 > 100 {
			gym.TotalCp = null.IntFrom(int64(fortData.GymDisplay.TotalGymCp))
		}
	}

	if fortData.RaidInfo != nil {
		gym.RaidEndTimestamp = null.IntFrom(int64(fortData.RaidInfo.RaidEndMs) / 1000)
		gym.RaidSpawnTimestamp = null.IntFrom(int64(fortData.RaidInfo.RaidSpawnMs) / 1000)
		gym.RaidBattleTimestamp = null.IntFrom(int64(fortData.RaidInfo.RaidBattleMs) / 1000)
		gym.RaidLevel = null.IntFrom(int64(fortData.RaidInfo.RaidLevel))
		if fortData.RaidInfo.RaidPokemon != nil {
			gym.RaidPokemonId = null.IntFrom(int64(fortData.RaidInfo.RaidPokemon.PokemonId))
			gym.RaidPokemonMove1 = null.IntFrom(int64(fortData.RaidInfo.RaidPokemon.Move1))
			gym.RaidPokemonMove2 = null.IntFrom(int64(fortData.RaidInfo.RaidPokemon.Move2))
			gym.RaidPokemonForm = null.IntFrom(int64(fortData.RaidInfo.RaidPokemon.PokemonDisplay.Form))
			gym.RaidPokemonCp = null.IntFrom(int64(fortData.RaidInfo.RaidPokemon.Cp))
			gym.RaidPokemonGender = null.IntFrom(int64(fortData.RaidInfo.RaidPokemon.PokemonDisplay.Gender))
			gym.RaidPokemonCostume = null.IntFrom(int64(fortData.RaidInfo.RaidPokemon.PokemonDisplay.Costume))
			gym.RaidPokemonEvolution = null.IntFrom(int64(fortData.RaidInfo.RaidPokemon.PokemonDisplay.CurrentTempEvolution))
		} else {
			gym.RaidPokemonId = null.IntFrom(0)
			gym.RaidPokemonMove1 = null.IntFrom(0)
			gym.RaidPokemonMove2 = null.IntFrom(0)
			gym.RaidPokemonForm = null.IntFrom(0)
			gym.RaidPokemonCp = null.IntFrom(0)
			gym.RaidPokemonGender = null.IntFrom(0)
			gym.RaidPokemonCostume = null.IntFrom(0)
			gym.RaidPokemonEvolution = null.IntFrom(0)
		}

		gym.RaidIsExclusive = null.IntFrom(util.BoolToInt[int64](fortData.RaidInfo.IsExclusive))
	}

	gym.CellId = null.IntFrom(int64(cellId))

	return gym
}

func (gym *Gym) updateGymFromFortProto(fortData *pogo.FortDetailsOutProto) *Gym {
	gym.Id = fortData.Id
	gym.Lat = fortData.Latitude  //fmt.Sprintf("%f", fortData.Latitude)
	gym.Lon = fortData.Longitude //fmt.Sprintf("%f", fortData.Longitude)
	if len(fortData.ImageUrl) > 0 {
		gym.Url = null.StringFrom(fortData.ImageUrl[0])
	}
	gym.Name = null.StringFrom(fortData.Name)

	return gym
}

func (gym *Gym) updateGymFromGymInfoOutProto(gymData *pogo.GymGetInfoOutProto) *Gym {
	//gym.Id = gymData.Fo
	//gym.Lat = gymData.Latitude
	//gym.Lon = gymData.Longitude
	// This will have gym defenders in it...
	if len(gymData.Url) > 0 {
		gym.Url = null.StringFrom(gymData.Url)
	}
	gym.Name = null.StringFrom(gymData.Name)

	return gym
}

func hasChangesGym(old *Gym, new *Gym) bool {
	return !cmp.Equal(old, new, ignoreNearFloats)
}

type GymDetailsWebhook struct {
	Id                  string  `json:"id"`
	Name                string  `json:"name"`
	Url                 string  `json:"url"`
	Latitude            float64 `json:"latitude"`
	Longitude           float64 `json:"longitude"`
	Team                int64   `json:"team"`
	GuardPokemonId      int64   `json:"guard_pokemon_id"`
	SlotsAvailable      int64   `json:"slots_available"`
	ExRaidEligible      int64   `json:"ex_raid_eligible"`
	InBattle            bool    `json:"in_battle"`
	SponsorId           int64   `json:"sponsor_id"`
	PartnerId           int64   `json:"partner_id"`
	PowerUpPoints       int64   `json:"power_up_points"`
	PowerUpLevel        int64   `json:"power_up_level"`
	PowerUpEndTimestamp int64   `json:"power_up_end_timestamp"`
	ArScanEligible      int64   `json:"ar_scan_eligible"`

	//"id": id,
	//"name": name ?? "Unknown",
	//"url": url ?? "",
	//"latitude": lat,
	//"longitude": lon,
	//"team": teamId ?? 0,
	//"guard_pokemon_id": guardPokemonId ?? 0,
	//"slots_available": availableSlots ?? 6,
	//"ex_raid_eligible": exRaidEligible ?? 0,
	//"in_battle": inBattle ?? false,
	//"sponsor_id": sponsorId ?? 0,
	//"partner_id": partnerId ?? 0,
	//"power_up_points": powerUpPoints ?? 0,
	//"power_up_level": powerUpLevel ?? 0,
	//"power_up_end_timestamp": powerUpEndTimestamp ?? 0,
	//"ar_scan_eligible": arScanEligible ?? 0
}

func createGymWebhooks(oldGym *Gym, gym *Gym) {
	if oldGym == nil ||
		(oldGym.AvailableSlots != gym.AvailableSlots || oldGym.TeamId != gym.TeamId || oldGym.InBattle != gym.InBattle) {
		gymDetails := GymDetailsWebhook{
			Id:             gym.Id,
			Name:           gym.Name.ValueOrZero(),
			Url:            gym.Url.ValueOrZero(),
			Latitude:       gym.Lat,
			Longitude:      gym.Lon,
			Team:           gym.TeamId.ValueOrZero(),
			GuardPokemonId: gym.GuardingPokemonId.ValueOrZero(),
			SlotsAvailable: func() int64 {
				if gym.AvailableSlots.Valid {
					return gym.AvailableSlots.Int64
				} else {
					return 6
				}
			}(),
			ExRaidEligible: gym.ExRaidEligible.ValueOrZero(),
			InBattle:       func() bool { return gym.InBattle.ValueOrZero() != 0 }(),
		}

		webhooks.AddMessage(webhooks.GymDetails, gymDetails)
	}

	if gym.RaidSpawnTimestamp.ValueOrZero() > 0 &&
		(oldGym == nil || oldGym.RaidLevel != gym.RaidLevel ||
			oldGym.RaidPokemonId != gym.RaidPokemonId ||
			oldGym.RaidSpawnTimestamp != gym.RaidSpawnTimestamp) {
		raidBattleTime := gym.RaidBattleTimestamp.ValueOrZero()
		raidEndTime := gym.RaidEndTimestamp.ValueOrZero()
		now := time.Now().Unix()

		if (raidBattleTime > now && gym.RaidLevel.ValueOrZero() > 0) ||
			(raidEndTime > now && gym.RaidPokemonId.ValueOrZero() > 0) {
			raidHook := map[string]interface{}{
				"gym_id": gym.Id,
				"gym_name": func() string {
					if !gym.Name.Valid {
						return "Unknown"
					} else {
						return gym.Name.String
					}
				}(),
				"gym_url":                gym.Url.ValueOrZero(),
				"latitude":               gym.Lat,
				"longitude":              gym.Lon,
				"team_id":                gym.TeamId.ValueOrZero(),
				"spawn":                  gym.RaidSpawnTimestamp.ValueOrZero(),
				"start":                  gym.RaidBattleTimestamp.ValueOrZero(),
				"end":                    gym.RaidEndTimestamp.ValueOrZero(),
				"level":                  gym.RaidLevel.ValueOrZero(),
				"pokemon_id":             gym.RaidPokemonId.ValueOrZero(),
				"cp":                     gym.RaidPokemonCp.ValueOrZero(),
				"gender":                 gym.RaidPokemonGender.ValueOrZero(),
				"form":                   gym.RaidPokemonForm.ValueOrZero(),
				"costume":                gym.RaidPokemonCostume.ValueOrZero(),
				"evolution":              gym.RaidPokemonEvolution.ValueOrZero(),
				"move_1":                 gym.RaidPokemonMove1.ValueOrZero(),
				"move_2":                 gym.RaidPokemonMove2.ValueOrZero(),
				"ex_raid_eligible":       gym.ExRaidEligible.ValueOrZero(),
				"is_exclusive":           gym.RaidIsExclusive.ValueOrZero(),
				"sponsor_id":             gym.SponsorId.ValueOrZero(),
				"partner_id":             gym.PartnerId.ValueOrZero(),
				"power_up_points":        gym.PowerUpPoints.ValueOrZero(),
				"power_up_level":         gym.PowerUpLevel.ValueOrZero(),
				"power_up_end_timestamp": gym.PowerUpEndTimestamp.ValueOrZero(),
				"ar_scan_eligible":       gym.ArScanEligible.ValueOrZero(),
			}

			webhooks.AddMessage(webhooks.Raid, raidHook)
		}
	}

}

func saveGymRecord(db *sqlx.DB, gym *Gym) {
	oldGym, _ := getGymRecord(db, gym.Id)

	if oldGym != nil && !hasChangesGym(oldGym, gym) {
		return
	}

	log.Traceln(cmp.Diff(oldGym, gym))
	if oldGym == nil {
		res, err := db.NamedExec("INSERT INTO gym (id,lat,lon,name,url,last_modified_timestamp,raid_end_timestamp,raid_spawn_timestamp,raid_battle_timestamp,updated,raid_pokemon_id,guarding_pokemon_id,available_slots,team_id,raid_level,enabled,ex_raid_eligible,in_battle,raid_pokemon_move_1,raid_pokemon_move_2,raid_pokemon_form,raid_pokemon_cp,raid_is_exclusive,cell_id,deleted,total_cp,first_seen_timestamp,raid_pokemon_gender,sponsor_id,partner_id,raid_pokemon_costume,raid_pokemon_evolution,ar_scan_eligible,power_up_level,power_up_points,power_up_end_timestamp) "+
			"VALUES (:id,:lat,:lon,:name,:url,UNIX_TIMESTAMP(),:raid_end_timestamp,:raid_spawn_timestamp,:raid_battle_timestamp,UNIX_TIMESTAMP(),:raid_pokemon_id,:guarding_pokemon_id,:available_slots,:team_id,:raid_level,:enabled,:ex_raid_eligible,:in_battle,:raid_pokemon_move_1,:raid_pokemon_move_2,:raid_pokemon_form,:raid_pokemon_cp,:raid_is_exclusive,:cell_id,0,:total_cp,UNIX_TIMESTAMP(),:raid_pokemon_gender,:sponsor_id,:partner_id,:raid_pokemon_costume,:raid_pokemon_evolution,:ar_scan_eligible,:power_up_level,:power_up_points,:power_up_end_timestamp)", gym)

		if err != nil {
			log.Printf("insert gym: %s", err)
			return
		}

		_, _ = res, err
	} else {
		res, err := db.NamedExec("UPDATE gym SET "+
			"lat = :lat, "+
			"lon = :lon, "+
			"name = :name, "+
			"url = :url, "+
			"last_modified_timestamp = :last_modified_timestamp, "+
			"raid_end_timestamp = :raid_end_timestamp, "+
			"raid_spawn_timestamp = :raid_spawn_timestamp, "+
			"raid_battle_timestamp = :raid_battle_timestamp, "+
			"updated = UNIX_TIMESTAMP(), "+
			"raid_pokemon_id = :raid_pokemon_id, "+
			"guarding_pokemon_id = :guarding_pokemon_id, "+
			"available_slots = :available_slots, "+
			"team_id = :team_id, "+
			"raid_level = :raid_level, "+
			"enabled = :enabled, "+
			"ex_raid_eligible = :ex_raid_eligible, "+
			"in_battle = :in_battle, "+
			"raid_pokemon_move_1 = :raid_pokemon_move_1, "+
			"raid_pokemon_move_2 = :raid_pokemon_move_2, "+
			"raid_pokemon_form = :raid_pokemon_form, "+
			"raid_pokemon_cp = :raid_pokemon_cp, "+
			"raid_is_exclusive = :raid_is_exclusive, "+
			"cell_id = :cell_id, "+
			"deleted = :deleted, "+
			"total_cp = :total_cp, "+
			"raid_pokemon_gender = :raid_pokemon_gender, "+
			"sponsor_id = :sponsor_id, "+
			"partner_id = :partner_id, "+
			"raid_pokemon_costume = :raid_pokemon_costume, "+
			"raid_pokemon_evolution = :raid_pokemon_evolution, "+
			"ar_scan_eligible = :ar_scan_eligible, "+
			"power_up_level = :power_up_level, "+
			"power_up_points = :power_up_points, "+
			"power_up_end_timestamp = :power_up_end_timestamp "+
			"WHERE id = :id", gym,
		)
		if err != nil {
			log.Printf("Update gym %s", err)
		}
		_, _ = res, err
	}

	gymCache.Set(gym.Id, *gym, ttlcache.DefaultTTL)
	createGymWebhooks(oldGym, gym)
}

func UpdateGymRecordWithFortDetailsOutProto(db *sqlx.DB, fort *pogo.FortDetailsOutProto) string {
	gym, err := getGymRecord(db, fort.Id) // should check error
	if err != nil {
		panic(err)
	}

	if gym == nil {
		gym = &Gym{}
	}
	gym.updateGymFromFortProto(fort)
	saveGymRecord(db, gym)

	return fmt.Sprintf("%s %s", gym.Id, gym.Name.ValueOrZero())
}

func UpdateGymRecordWithGymInfoProto(db *sqlx.DB, gymInfo *pogo.GymGetInfoOutProto) string {
	gym, err := getGymRecord(db, gymInfo.GymStatusAndDefenders.PokemonFortProto.FortId) // should check error
	if err != nil {
		panic(err)
	}

	if gym == nil {
		gym = &Gym{}
	}
	gym.updateGymFromGymInfoOutProto(gymInfo)
	saveGymRecord(db, gym)
	return fmt.Sprintf("%s %s", gym.Id, gym.Name.ValueOrZero())
}
